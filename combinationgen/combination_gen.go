package combinationgen

import (
	"fmt"
	"io"
)

//The general idea is that we keep a counter into what will currently be incremented,
//and a counter for each vprops. 
//like we have a x and y axis.
//Each time, we increment y axis. When we reach end of any y, we increent x and may
//go backwards.

//Generates combinations of everything in the combo slice of slices.
type T struct {
	vprops []interface{}
	combo [][]interface{}
	ictr int
	ctr []int
	err error
}

func New(vprops []interface{}, combo [][]interface{}) (cg *T, err error) {
	if len(vprops) == 0 || len(vprops) != len(combo) {
		err = fmt.Errorf("slices must be same length and not 0. vprops: %v, combo: %v", len(vprops), len(combo))
		return
	}
	for i, c := range combo {
		if len(c) == 0 {
			err = fmt.Errorf("Found a combo element with zero length. Index: %v", i)
			return
		}
	}
	cg = &T{
		vprops: vprops,
		combo:combo, 
		ctr: make([]int, len(vprops)),
	}
	return
}

func (x *T) First() *T {
	for i := 0; i < len(x.vprops); i++ {
		x.vprops[i] = x.combo[i][0]
	}
	x.ictr = 0
	return x
}


func (x *T) Next() (err error) {
	if x.err != nil {
		return x.err
	}
	//whenever we go back and reach -1, we're done
	for {
		//println("ictr: ", x.ictr, ", x.ctr[x.ictr]: ", x.ctr[x.ictr])
		//try to move forward
		if x.ictr < len(x.vprops)-1 {
			x.ictr++
			x.upd2(0)
			continue
		}
		//if not try to write current if not past end of me
		if x.ctr[x.ictr] < len(x.combo[x.ictr])-1 {
			x.ctr[x.ictr]++
			x.upd()
			return
		}
		//if not, move back and continue
		//need to find appropriate person to move back to
		for {
			x.upd2(0)
			x.ictr--
			if x.ictr < 0 {
				x.err = io.EOF
				break
			} 
			//println("== ictr: ", x.ictr, ", x.ctr[x.ictr]: ", x.ctr[x.ictr])
			if x.ctr[x.ictr] == len(x.combo[x.ictr])-1 {
				continue
			}
			x.ctr[x.ictr]++
			x.upd()
			break
		}
		break
	}
	return x.err
}


func (x *T) upd() {
	x.vprops[x.ictr] = x.combo[x.ictr][x.ctr[x.ictr]]
}

func (x *T) upd2(comboIndx int) {
	x.ctr[x.ictr] = comboIndx
	x.upd()
}

