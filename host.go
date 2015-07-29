

package fleet

import (
    "fmt"
    "strings"
    "strconv"
)


type Login struct{
    Username    string
    Password    string
    KeyPassword string
    KeyPublic   string
    KeyPrivate  string
}

type Host struct{
    Hostname   string
    Cpus       float64
    Benchmark  float64
    Memory     float64
    Load1      float64
    Load5      float64
    Load15     float64
    Memutil    float64
    Wait       float64
    Logins     map[string]Login
}

func NewHost(name string)*Host{
    h := new(Host)
    h.Hostname = name
    h.Logins = make(map[string]Login)
    return h
}

func (h *Host) AddLogin (username, password string){
    h.Logins[username] = Login{Username:username, Password:password}
}

func (h *Host) GetHostStatus (){
    i:=h.GetInterp("bash")
    status := getInfo(i)
    h.Cpus      = status["#cpu"]
    h.Benchmark = status["bench"]
    h.Memory    = status["memtotal"]
    h.Load1     = status["load1"]
    h.Load5     = status["load5"]
    h.Load15    = status["load15"]
    h.Memutil   = status["memutil"]
    h.Wait      = status["wait"]
    i.Close()
}

func (h *Host)ShowHost(){
    fmt.Printf("Hostname: %s\n", h.Hostname)
    fmt.Printf("CPUs: %f\n", h.Cpus)
    fmt.Printf("Memory: %f\n", h.Memory)
    fmt.Printf("Load 1 : %f\n", h.Load1)
    fmt.Printf("Load 5 : %f\n", h.Load5)
    fmt.Printf("Load 15: %f\n", h.Load15)
    fmt.Printf("Wait: %f\n", h.Wait)
    fmt.Printf("Mem Util: %f\n", h.Memutil)
    fmt.Printf("\nLogins\n")

    for _,x := range h.Logins{
        fmt.Printf("    %s\n", x.Username)
        fmt.Printf("        pass: %s\n", x.Password)
        //~ fmt.Printf("        key : %s\n", x.Key)
    }
    fmt.Println()
}

func (h *Host)GetInterp(lang string) *Interp{
    var login Login
    for _,x := range h.Logins{
        login = x
        break
    }
    return NewInterp(lang, h.Hostname, login.Username, login.Password, login.KeyPrivate)
}

func (h *Host)GetInterpAsUser(lang, user string) *Interp{
    login := h.Logins[user]    
    return NewInterp(lang, h.Hostname, login.Username, login.Password, login.KeyPrivate)
}


func getStat(i *Interp) map[string]float64{
    var usertime float64
    var iowait   float64
    var a []string

    i.Request("cat /proc/stat")
    data1 := strings.Split(i.Stdout, "\n")
    for _,line := range data1{
        line = strings.ToLower(line)
        if strings.HasPrefix(line, "cpu "){
            a = splitTrim(line, "", " \r\n\t")
            usertime,_ = strconv.ParseFloat(a[1], 64)
            iowait,_ = strconv.ParseFloat(a[5], 64)
            break
        }
    }
    return map[string]float64{"usertime":usertime, "iowait":iowait}
}


func getCpuinfo(i *Interp) map[string]float64{
    processors := 0.0
    i.Request("cat /proc/cpuinfo")
    data1 := strings.Split(i.Stdout, "\n")    
    for _,line := range data1{
        line = strings.ToLower(line)
        if strings.HasPrefix(line, "processor"){
            processors++;
        }
    }
    return map[string]float64{"#cpu":processors}
}


func getLoadavg(i *Interp, num_procs float64) map[string]float64{
    i.Request("cat /proc/loadavg")
    data1 := strings.Split(i.Stdout, " ")
    load :=func(s string)float64{
        ll,err  := strconv.ParseFloat(s,64)
            if err != nil{return -1.0}
        return ll/float64(num_procs)
    }

    return map[string]float64{
        "load1":load(data1[0]), 
        "load5":load(data1[1]), 
        "load15":load(data1[2]), 
        }
}


func getMeminfo(i *Interp)map[string]float64{
    i.Request("cat /proc/meminfo")
    data1 := strings.Split(i.Stdout, "\n")
    memtotal     := 0.0
    memfree      := 0.0
    meminactive  := 0.0
    
    for _,line := range data1{
        line = strings.ToLower(line)
        if strings.HasPrefix(line, "memtotal:"){
            memtotal,_ = strconv.ParseFloat(splitTrim(line, ":", " \r\n\t kb")[1],64)
        }else if strings.HasPrefix(line, "memfree:"){
            memfree,_ = strconv.ParseFloat(splitTrim(line, ":", " \r\n\t kb")[1],64)
        }else if strings.HasPrefix(line, "inactive:"){
            meminactive,_ = strconv.ParseFloat(splitTrim(line, ":", " \r\n\t kb")[1],64)
        }
    }    

    return map[string]float64{
        "memtotal": memtotal * 1000.0,
        "memutil":1.0 - (float64(memfree + meminactive) / float64(memtotal)),
    }
}


func getBench(i *Interp)map[string]float64{
    bench :=`
        start=$(date +%s%N)
        a=0
        while [[ $a -le 10000 ]]
        do
            a=$(( $a+1 ))
            elapsed=$(( $(date +%s%N)-$start ))
            if [[ $elapsed -gt  1000000000 ]];then
                break
            fi
        done
        echo -n $a
    `
    i.Request(bench)
    b,_ := strconv.ParseFloat(i.Stdout,64)
    return map[string]float64{
        "bench":b,
    }
}


func getInfo(i *Interp) map[string]float64{

    update := func(a,b map[string]float64){
        for k,v := range b{
            a[k] = v
        }
    }

    info := make(map[string]float64)

    stat_a  := getStat(i)
    update( info, getBench(i) )
    update( info, getCpuinfo(i) )
    update( info, getLoadavg(i, info["#cpu"]) )
    update( info, getMeminfo(i) )
    stat_b  := getStat(i)

    ut := stat_b["usertime"] - stat_a["usertime"]
    wt := stat_b["iowait"] - stat_a["iowait"]
    
    if ut+wt > 0{
        info["wait"] =  wt/(ut+wt)
    }else{
        info["wait"] = 0.0
    }

    return info
}
