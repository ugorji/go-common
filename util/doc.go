/*
Utilities: containers, types, etc.

It includes utility functions and utility types which across the codebase.

The utility types include:
   - tree (Int64, interface{})
   - bitset
   - combination generator
   - cron definition and support
   - buffered byte reader which does not include copying (as compared to bytes.Buffer)
   - lock set
   - safe store
   - virtual file system

*/
package util

/*
Some guidelines:
  - All packages include this. 
    Consequently, be careful about packages this depends on:
    - do not include net/http. It is too big.
      http helpers should be in web.
    
*/
