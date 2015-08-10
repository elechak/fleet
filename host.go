

package fleet

import (
    "fmt"
    "strings"
    "strconv"
    "encoding/json"
    "io/ioutil"
    "sync"
    "sort"
)


type Host struct{
    Hostname    string
    Cpus        float64
    Benchmark   float64
    Memory      float64
    Load1       float64
    Load5       float64
    Load15      float64
    Memutil     float64
    Wait        float64
    Username    string
    Password    string
    Keypassword string
    Keypublic   string
    Keyprivate  string
    
    ACpus float64
    AMemory float64
    ABenchmark float64    
}

type Group struct{
    Name string
    Hosts     map[string]*Host
}


// GROUP
func NewGroup( name string )*Group{
    self := new(Group)
    self.Name = name
    self.Hosts = make(map[string]*Host)
    return self
}

func LoadGroup( filename string) *Group{
    var group Group
    s,_ := ioutil.ReadFile(filename)
    json.Unmarshal(s, &group)
    return &group
}

func (self *Group) Save(filename string){
    s,_ := json.Marshal(self)
    ioutil.WriteFile(filename, []byte(s), 0644)
}

func (self *Group) AddHost(hostname string) *Host{
    h := NewHost(hostname)
    self.Hosts[hostname] =  h
    return h
}

func (self *Group) Host( hostname string) *Host{
    return self.Hosts[hostname]
}

func (self *Group) List(){
    var a []string
    for k,_ := range self.Hosts{
        a=append(a,k)
    }
    fmt.Println(a)
}

func (self *Group) GetStatus(){
    var wg sync.WaitGroup
    for _,v := range self.Hosts{
        wg.Add(1)
        go func(v *Host){
            defer wg.Done()
            v.GetStatus()
        }(v)
    }
    wg.Wait()
}

func (self *Group) Show(){
    for _,v := range self.Hosts{
        v.Show()
    }
}

type byResource []*Host
func (self byResource) Len() int {return len(self)}
func (self byResource) Swap(a,b int){self[a],self[b] = self[b],self[a]}
func (self byResource) Less(a,b int) bool { return self[a].ABenchmark < self[b].ABenchmark }


func (self *Group) Pool(lang string, max int,mem_requirement float64) (interps []*Interp){
    var out []*Host
    var res []*Host
    var tmp []*Host
    
    for _,v := range self.Hosts{
        res = append(res, v )
    }

    for{
        tmp = []*Host{}
        // filter 
        for _,r := range res{
            if r.AMemory < mem_requirement{continue}
            if r.ACpus <= 0.0 {continue}
            tmp = append(tmp, r)
        }

        if len(tmp) == 0 { break }

        sort.Sort(sort.Reverse(byResource(tmp)))
        
        //~ for _,r := range tmp{
            //~ fmt.Println("-------")
            //~ fmt.Println(r)
        //~ }
        //~ fmt.Println("++++++++++")
        
        
        r := tmp[0]
        //~ fmt.Println(r)
        r.ACpus -= 1.0
        r.AMemory -= mem_requirement
        r.ABenchmark -= r.ABenchmark * 0.1
        out = append(out,r)
        if len(out) == max {break}
        res = tmp
    }

    for _,r := range out{
        interps = append(interps,self.Hosts[r.Hostname].GetInterp(lang) )
    }
    return interps
}




// HOST

func NewHost(name string)*Host{
    h := new(Host)
    h.Hostname = name
    return h
}

func (self *Host) Login(username, password string){
    self.Username = username
    self.Password = password
}

func (h *Host) GetStatus(){
    i := h.GetInterp("bash")
    status := getInfo(i)
    i.Close()
    h.Cpus      = status["#cpu"]
    h.Benchmark = status["bench"]
    h.Memory    = status["memtotal"]
    h.Load1     = status["load1"]
    h.Load5     = status["load5"]
    h.Load15    = status["load15"]
    h.Memutil   = status["memutil"]
    h.Wait      = status["wait"]
    
    h.ACpus    = h.Cpus
    h.AMemory  = (1.0 - h.Memutil) * h.Memory
    adj_load := 1.0 - h.Load1
    adj_wait := 1.0 - h.Wait
    h.ABenchmark = h.Benchmark * adj_load * adj_wait    
}

func (h *Host)Show(){
    fmt.Printf("Hostname: %s\n", h.Hostname)
    fmt.Printf("Username: %s\n", h.Username)
    fmt.Printf("Password: %s\n", h.Password)
    fmt.Printf("Keypassword: %s\n", h.Keypassword)
    fmt.Printf("Keypublic: %s\n", h.Keypublic)
    fmt.Printf("Keyprivate: %s\n", h.Keyprivate)
    
    fmt.Printf("CPUs: %f\n", h.Cpus)
    fmt.Printf("Benchmark: %f\n", h.Benchmark)
    fmt.Printf("Memory: %f\n", h.Memory)
    fmt.Printf("Load 1 : %f\n", h.Load1)
    fmt.Printf("Load 5 : %f\n", h.Load5)
    fmt.Printf("Load 15: %f\n", h.Load15)
    fmt.Printf("Wait: %f\n", h.Wait)
    fmt.Printf("Mem Util: %f\n", h.Memutil)

    fmt.Printf("Avail Mem: %f\n", h.AMemory)
    fmt.Printf("Avail CPUs: %f\n", h.ACpus)
    fmt.Printf("Avail Bench: %f\n", h.ABenchmark)

    fmt.Println()
    fmt.Println()
}

func (h *Host)GetInterp(lang string) *Interp{
    return NewInterp(lang, h.Hostname, h.Username, h.Password, h.Keyprivate)
}



// INTERP

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
            if len(a) >= 6{
                iowait,_ = strconv.ParseFloat(a[5], 64)
            }else{
                iowait = 0.0
            }
            
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
        "memtotal": memtotal / 1000000.0,
        "memutil":1.0 - (float64(memfree + meminactive) / float64(memtotal)),
    }
}


func getBench(i *Interp)map[string]float64{
    bench :=`
        read -r line < "/proc/uptime"
        x=(${line//./})
        start="${x[0]}"
        a=0
        while [[ $a -le 100000 ]]
        do
            a=$(( $a+1 ))    
            read -r line < "/proc/uptime"
            x=(${line//./})
            now="${x[0]}"
            elapsed=$(( $now-$start))
            if [[ $elapsed -ge  100 ]];then
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
