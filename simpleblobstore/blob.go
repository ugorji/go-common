package simpleblobstore

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ugorji/go-serverapp/app"
	"github.com/ugorji/go-common/errorutil"
)

var (
	blobBO  = binary.BigEndian //must be big endian, since we move upwards
	blobSeq uint64
	random  = rand.New(rand.NewSource(1 << 31))
)

// BlobDriver manages all blob handling on the backend.
//
// For now, blobdir and datadir must be on a shared drive (e.g. NFS mounted).
// All backends can write to them simultaneously.
// There might be some issues, but it's best to keep it simple for now.
//
// Blob handling is decentralized. Multiple backends do not have to
// use a single master.
//
// Blobs can be referred to by a long key string, or by a uint64.
//   - With a uint64, it just contains a reference to enough information
//     to get to that file.
//   - With a long key string, that key contains the uint64 id along with
//     all information about the blob (ie timestamp, content type, etc).
//
// We typically do not use the uint64 at this time. We don't have a clean
// way of checking if the file is unique.
//
// The key is a base64 encoding of:
//    shardL1(1) shardL2(1) shardL3(1) random(5)
//    size(8) creationTimeSec(8) creationTimeNs(4) contentType(n)
// Note that the top gives a unique representation of the blob,
// and bottom are just metadata.
//
// On the file system, the files will be in the directory structure:
// (shardL1, shardL2 and shardL3 are just the 3 level directories below).
//   BLOBDIR/
//     n(00-ff)/      (configurable number of sub-directories at this level)
//       n(00-ff)/    (configurable number of sub-directories at this level)
//         n(00-ff)/  (configurable number of sub-directories at this level)
//           (00-ff)5-ZZZZZZZZZZZZZZZZZZ (files here)
//
// Note that we do not store blobs onto a flat directory. This has issues with reaching
// the maximum sub-directory depth and number of files in a directory. Even though
// most filesystems can now handle immense number of files, tools like ls, du, etc
// will blow up.
//
// With the model we have, we have 3 levels of subdirectories each having up to
// 256 directories below them. This means we can have up to 256^3 directories
// containing files (i.e. 16 million sub directories).
//
// However, for the same reasons as above, we'd stop at about 1000
// children per directory, giving us up to 16 billion entities to store
// (with room to grow).
type BlobDriver struct {
	Dir string
}

type nblobw struct {
	dir string
	bi  *app.BlobInfo
	*os.File
}

type blobKeyString string

type blobKeyBytes []byte

func (k blobKeyString) Bytes() (bs []byte, err error) {
	x := string(k)
	bs = make([]byte, 8)
	j, err := strconv.ParseUint(x[:16], 16, 8)
	if err != nil {
		return
	}
	blobBO.PutUint64(bs[0:], j)
	// index 16 is a -
	kbs, err := base64.URLEncoding.DecodeString(x[17:])
	if err != nil {
		return
	}
	bs = append(bs, kbs...)
	return
}

func (k blobKeyString) Location(basedir string) (dir, file string) {
	x := string(k)
	dir = filepath.Join(basedir, x[0:2], x[2:4], x[4:6])
	file = x[6:]
	return
}

func (x blobKeyBytes) String() (s string) {
	bs := make([]byte, 17)
	for i := 0; i < 16; i += 2 {
		switch s2 := strconv.FormatUint(uint64(x[i/2]), 16); len(s2) {
		case 2:
			bs[i], bs[i+1] = s2[0], s2[1]
		case 1:
			bs[i], bs[i+1] = '0', s2[0]
		default:
			panic(fmt.Sprintf("blobKeyBytes.String has len: %v", len(s2)))
		}
	}
	bs[16] = '-'
	bs = append(bs, base64.URLEncoding.EncodeToString(x[8:])...)
	return string(bs)
}

func (bs blobKeyBytes) LoadBlobInfo(bi *app.BlobInfo) {
	bi.Size = int64(blobBO.Uint64(bs[8:]))
	bi.CreationTime = time.Unix(
		int64(blobBO.Uint64(bs[16:])),
		int64(blobBO.Uint32(bs[24:])))
	bi.ContentType = string(bs[28:])
}

func (bs blobKeyBytes) BlobId() uint64 {
	return blobBO.Uint64(bs[0:8])
}

func (w nblobw) Finish() (key string, err error) {
	defer errorutil.OnError(&err)
	if err = w.Close(); err != nil {
		return
	}
	//fi, err := w.Stat()
	fi, err := os.Stat(w.Name())
	if err != nil {
		return
	}
	w.bi.Size = fi.Size()

	bs := blobInfoToKey(w.bi)
	w.bi.Key = blobKeyBytes(bs).String()
	//if err = os.Rename(w.Name(), filepath.Join(w.blobdir, w.bi.Key)); err != nil {return}
	reldir, blobf := blobKeyString(w.bi.Key).Location(w.dir)
	if err = os.MkdirAll(reldir, 0777); err != nil {
		return
	}
	if err = os.Rename(w.Name(), filepath.Join(reldir, blobf)); err != nil {
		return
	}
	key = w.bi.Key
	return
}

func (l BlobDriver) BlobWriter(ctx app.Context, contentType string,
) (b app.BlobWriter, err error) {
	defer errorutil.OnError(&err)
	bi := &app.BlobInfo{
		ContentType:  contentType,
		CreationTime: time.Now(),
	}
	//if bi.Filename, err = util.UUID(16); err != nil {
	//	return
	//}
	tempf, err := ioutil.TempFile(l.Dir, "ndb-blobw-tmp")
	if err != nil {
		return
	}
	b = &nblobw{l.Dir, bi, tempf}
	return
}

func (l BlobDriver) BlobReader(ctx app.Context, key string) (br app.BlobReader, err error) {
	defer errorutil.OnError(&err)
	reldir, blobf := blobKeyString(key).Location(l.Dir)
	f, err := os.Open(filepath.Join(reldir, blobf))
	if err != nil {
		return
	}
	br = f
	return
}

func (l BlobDriver) BlobInfo(ctx app.Context, key string) (bi *app.BlobInfo, err error) {
	defer errorutil.OnError(&err)
	bs, err := blobKeyString(key).Bytes()
	if err != nil {
		return
	}
	bi = new(app.BlobInfo)
	bi.Key = key
	blobKeyBytes(bs).LoadBlobInfo(bi)
	return
}

func (l BlobDriver) BlobServe(c app.Context, key string,
	response http.ResponseWriter) (err error) {
	defer errorutil.OnError(&err)
	f, err := os.Open(filepath.Join(l.Dir, key))
	if err != nil {
		return
	}
	defer f.Close()
	if _, err = io.Copy(response, f); err != nil {
		return
	}
	return
}

func blobInfoToKey(bi *app.BlobInfo) (bs []byte) {
	//random(8), size(8), creationTimeSec(8), creationTimeNs(4), contentType(n)
	bs = make([]byte, 28+len(bi.ContentType))
	blobBO.PutUint64(bs[0:], blobRand())
	blobBO.PutUint64(bs[8:], uint64(bi.Size))
	blobBO.PutUint64(bs[16:], uint64(bi.CreationTime.Unix()))
	blobBO.PutUint32(bs[24:], uint32(bi.CreationTime.Nanosecond()))
	copy(bs[28:], bi.ContentType)
	return
}

func blobRand() uint64 {
	//the random gives us only the half range (ie 0-MaxInt)
	var v int64
	nbig, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err == nil {
		v = nbig.Int64()
	} else {
		v = random.Int63n(math.MaxInt64)
	}
	//to get full range, flip the sign bit every other time
	if v2 := atomic.AddUint64(&blobSeq, 1); v2%2 == 1 {
		//v ^= 1 << 63 //WRONG: doesn't work
		v = -v
	}

	return uint64(v)
}
