
package fleet

import (
    "strings"
)


func check(err error){
    if err != nil{
        panic(err)
    }
}

func splitTrim(s string, spliter string, trimmer string) (a []string){
    if spliter == ""{
        a = strings.Fields(s)
    }else{
        a = strings.Split(s, spliter)
    }
    
    for n,e := range a{
        a[n] = strings.Trim(e, trimmer)
    }
    return a
}