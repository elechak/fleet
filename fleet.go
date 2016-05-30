
package main

import (
    "golang.org/x/crypto/ssh"
    "fmt"
    "bytes"
    "time"
    "strings"
    "io"
    "io/ioutil"
    "os"
    "net"
    //~ "flag"
    "strconv"
    "math/rand"
    "encoding/json"
    "sync"
    "sort"
)

// Convenience functions

func ignore(x ...interface{}){}
func check(e ...error){
    for _,x := range e{
        if x != nil{fmt.Println(e)}
    }
}
func print(s ...interface{}){
    fmt.Println(s)
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

type Args struct {
    Words []string
    Flags map[string][]string
}

func ParseArgs() Args{
    var words []string
    flags := make(map[string][]string)
    args     := os.Args
    args_len := len(args)
    args_index := 1
    for args_index < args_len {
        arg := args[args_index]
        
        if strings.HasPrefix(arg, "-"){
            arg = arg[1:]
            args_index += 1
            
            a := strings.SplitN(arg, "=", 2)
            if len(a) == 2{
                flags[a[0]] = append(flags[a[0]], a[1])
            }else{
               flags[a[0]] = append(flags[a[0]], "")
            }
        }else{
            words = append(words, arg)
            args_index += 1
        }
    }
    
    return Args{words,flags}
}


// ###############################  Languages  #######################################

var langs map[string]string;

func init(){

langs = make(map[string]string)

langs["python3"] = `python3 -c '
import sys, traceback
while 1:
    __D = []
    while 1:
        __C = sys.stdin.read(1)
        if not __C: sys.exit()
        if __C == "\x04": break
        __D.append(__C)
    try:
        exec("".join(__D))
    except:
        print(traceback.format_exc(), file=sys.stderr)
    sys.stdout.write("\x04")
    sys.stderr.write("\x04")
    sys.stdout.flush()
    sys.stderr.flush()
'
`

langs["python"] = `python -c '
import sys, traceback
while 1:
    __D = []
    while 1:
        __C = sys.stdin.read(1)
        if not __C: sys.exit()
        if __C == "\x04": break
        __D.append(__C)
    try:
        exec("".join(__D))
    except SystemExit:
        raise
    except:
        print(traceback.format_exc(), file=sys.stderr)
    sys.stdout.write("\x04")
    sys.stderr.write("\x04")
    sys.stdout.flush()
    sys.stderr.flush()
'
`

langs["lua"] = `lua -e '
while true do
    __D = ""
    while true do
        __C = io.read(1)

        if __C=="\x04" then
            break
        end
        
        if __C==nil then
            os.exit()
        end
                
        __D = __D .. __C
        
    end

    __D, __C = loadstring(__D)
    if __C then
        io.stderr:write(__C)
    else
        __D, __C = pcall(__D)
        if __C then
            io.stderr:write(__C)
        end
    end
    io.stdout:write("\x04")
    io.stdout:flush()
    io.stderr:write("\x04")
    io.stderr:flush()    
end
'
`


langs["bash"] = `bash -c '
while :
do
read -r -d "\x04" __C;
[[ ${#__C} = 0 ]] && exit;
eval "$__C";
echo -n -e "\x04";
echo -n -e "\x04" 1>&2;
done
'
`

langs["bash-cp"] = `bash -c '
while :
do
read -r -d ":" filename
read -r -d ":" size
read -r -d "" -n "$size" data
read -r -n 1 __C
echo -n -e "$data"  > "$filename";
echo -n -e "\x04";
echo -n -e "\x04" 1>&2;
done
'
`


langs["docker-bash"] = `docker run --rm=true -i ubuntu /bin/bash -c '
while :
do
read -d "\x04" __C;
[[ ${#__C} = 0 ]] && exit;
eval "$__C";
echo -n -e "\x04";
echo -n -e "\x04" 1>&2;
done
'
`

langs["nodejs"] = `nodejs -e '
var __FS = require("fs");
var __B = new Buffer(1);
while(1){
    var __D = "";
    var __C = "\x01";
    while(1){
        if (! __FS.readSync(0, __B, 0, __B.length, null)) process.exit();
        __C = __B.toString("ascii");
        if (__C == "\x04") break;
        __D = __D + __C;
    }
    try{
        eval(__D);
    } catch (__C){
        console.error(__C.stack)
    }
    __B.write("\x04");
    __FS.writeSync(1, __B, 0, 1);
    __FS.writeSync(2, __B, 0, 1);
}
'
`

langs["docker-python3"] = `docker run --rm=true -i ubuntu /usr/bin/python3.4 -c '
import sys, traceback
while 1:
    __D = []
    while 1:
        __C = sys.stdin.read(1)
        if not __C: sys.exit()
        if __C == "\x04": break
        __D.append(__C)
    try:
        exec("".join(__D))
    except:
        print(traceback.format_exc(), file=sys.stderr)
    sys.stdout.write("\x04")
    sys.stderr.write("\x04")
    sys.stdout.flush()
    sys.stderr.flush()
'
`


langs["perl"] = `perl -w -e '
$|=1;
while(1){
    $__D = "";
    while(1){
        sysread(STDIN,$__C,1);
        if (! $__C) {exit 0;}
        if ($__C eq "\x04") {last;}
        $__D = $__D . $__C;
    }
    eval $__D;
    print "\x04";
    print STDERR "\x04";
}
'
`
}



// ###############################  Interpreter  #######################################


type Interp struct{
    lang string
    host string
    user string
    session *ssh.Session
    write_time int64
    Stdout string
    stdout_buffer bytes.Buffer
    stdout_read_time int64
    Stderr string
    stderr_buffer bytes.Buffer
    stderr_read_time int64
    Exit_status string
    exit_read_time int64
    stdin_buffer io.WriteCloser
}


func NewInterp(lang, host, user, password, key string) *Interp{

    i := new(Interp)
    i.lang = lang
    i.host = host
    i.user = user
    
    am := make([]ssh.AuthMethod, 0)
    
    config := &ssh.ClientConfig{User: user}
    
    if len(key) > 0{
        data,_ := ioutil.ReadFile(key)
        k, _ := ssh.ParsePrivateKey(data)
        am = append(am, ssh.PublicKeys(k))
    }
    
    if len(password) > 0{
        am = append(am, ssh.Password(password))
    }
    
    config.Auth = am

    addr := host + ":22"
    
    client, err := ssh.Dial("tcp", addr, config)
        if err != nil {fmt.Println(err)}

    session, err := client.NewSession()
        if err != nil {fmt.Println(err)}
    
    i.session = session

    session.Stdout = &i.stdout_buffer
    session.Stderr = &i.stderr_buffer
    
    i.stdin_buffer, _ = session.StdinPipe()
    
    err = session.Start(strings.Replace(langs[lang], `\x04`, "\x04", -1))
        if err != nil {fmt.Println(err)}

    go func(i *Interp){
        e := session.Wait()
        i.exit_read_time = time.Now().UnixNano()
        if (e != nil){
            xx := e.(*ssh.ExitError)
            yy := xx.Waitmsg
            i.Exit_status = fmt.Sprintf("%v", yy.ExitStatus())
        }
    }(i)

    return i
}

func (self *Interp) Close(){
    self.session.Close()
}

func (self *Interp) Write(data string){
    self.stdout_read_time = 0
    self.stderr_read_time = 0
    self.Stdout = ""
    self.Stderr = ""
    s:= strings.Join([]string{data, "\x04"}, "")
    _,_ = self.stdin_buffer.Write([]byte(s))
    self.write_time = time.Now().UnixNano()
}

func imax64(a,b int64) int64{
    if (a >= b){
        return a
    } else {
        return b
    }
}

func (self *Interp) Read() int64 {
    // stdout
    b := self.stdout_buffer.Bytes()
    i := bytes.Index(b, []byte{'\x04'} )
    if ( i != -1){
        s,_ := self.stdout_buffer.ReadBytes( '\x04')
        self.stdout_read_time = time.Now().UnixNano()
        if (i > 0 ){
            self.Stdout = string(s[0:i])
        }
    }

    // stderr
    b = self.stderr_buffer.Bytes()
    i = bytes.Index(b, []byte{'\x04'} )
    if ( i != -1){
        s,_ := self.stderr_buffer.ReadBytes( '\x04')
        self.stderr_read_time = time.Now().UnixNano()
        if (i >0 ){
            self.Stderr = string(s[0:i])
        }
    }
    
    if (self.stdout_read_time!=0 && self.stderr_read_time!=0){
        return imax64(self.stdout_read_time , self.stderr_read_time)
    }
    
    return self.exit_read_time
}

func (self *Interp) Status() string{
    stat := []string{self.lang, self.host, self.user,  self.Exit_status}
    return strings.Join(stat, " ")
}


func (self *Interp) Show(){
    fmt.Println("HOST: ", self.host)
    fmt.Println("USER: ", self.user)
    fmt.Println("LANG: ", self.lang)
    fmt.Println("STDOUT: ", self.Stdout)
    fmt.Println("STDERR: ", self.Stderr)
    fmt.Println("EXIT: ", self.Exit_status)
}

func (self *Interp) Time() int64{
    return self.Read() - self.write_time
}

func (self *Interp) Request(req string){
    self.Write(req)
    self.Wait(1.0)
}

func (self *Interp) Wait(delay float64){
    for {
        if self.Read() != 0 {break}
        time.Sleep(time.Duration(delay * float64(time.Second)))
    }
}



// ###############################  POOLS  #######################################


func Write(interps []*Interp, data string){
    for _, i := range interps{
        i.Write(data)
    }
}
    
func Read(interps []*Interp) []*Interp {
    var out []*Interp
    for _,i:= range interps{
        if i.Read() != 0 { out = append(out, i) }
    }
    return out
}

func Wait(interps []*Interp, delay float64){
    for len(Read(interps)) != len(interps) {Sleep(delay)}
}

func Request(interps []*Interp, data string){
    Write(interps,data)
    Wait(interps, 0.1)
}

func Show(interps []*Interp){
    for _,i:= range interps{
        i.Show()
    }    
}

func Sleep(delay float64){
    time.Sleep(time.Duration(delay * float64(time.Second)))
}



// ######################## GROUPS ###########################


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
    Hosts     map[string]*Host
}


// GROUP
func NewGroup()*Group{
    self := new(Group)
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
    //~ s,_ := json.Marshal(self)
    s,_ := json.MarshalIndent(self, "", "    ")
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
            fmt.Println(r.AMemory, r.ACpus)
            if r.AMemory < mem_requirement{continue}
            if r.ACpus <= 0.0 {continue}
            tmp = append(tmp, r)
        }

        if len(tmp) == 0 { break }

        sort.Sort(sort.Reverse(byResource(tmp)))
        
        r := tmp[0]
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


func (h *Host)showStatus() string{
    var s string =""
    s += fmt.Sprintf("CPUs: %f\n", h.Cpus)
    s += fmt.Sprintf("Benchmark: %f\n", h.Benchmark)
    s += fmt.Sprintf("Memory: %f\n", h.Memory)
    s += fmt.Sprintf("Load 1 : %f\n", h.Load1)
    s += fmt.Sprintf("Load 5 : %f\n", h.Load5)
    s += fmt.Sprintf("Load 15: %f\n", h.Load15)
    s += fmt.Sprintf("Wait: %f\n", h.Wait)
    s += fmt.Sprintf("Mem Util: %f\n", h.Memutil)
    s += fmt.Sprintf("Avail Mem: %f\n", h.AMemory)
    s += fmt.Sprintf("Avail CPUs: %f\n", h.ACpus)
    s += fmt.Sprintf("Avail Bench: %f\n", h.ABenchmark)
    return s
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



// ###################### MAIN ###############################
var interpreters map[int]*Interp
var groups map[string] *Group

var interp_id int

//~ var serve        = flag.Bool("s", false, "serve")
//~ var read_stdin   = flag.Bool("r", false, "read stdin")
//~ var message      = flag.String("c", "", "command")


func nextArg(message []byte, sep string) (string, []byte){
    message = bytes.TrimLeft(message, " \n\r\t")
    data := bytes.SplitN(message, []byte(sep), 2 )
    if len(data) == 2 {
        return string(data[0]), data[1]
    } else {
        return string(data[0]), []byte{}
    }
}



func doCommand(content []byte,c net.Conn){
    command, content := nextArg(content, " ")
    fmt.Println(command)
    
    // Start an interpreter
    // start bash localhost username password
    if command =="start"{ 
        fmt.Println(content)
        args := strings.Split(string(content), " ")
        r_type    := args[0]
        r_address := args[1]
        r_user    := args[2]
        r_pass    := args[3]
        r_key     := ""
        interp := NewInterp(r_type, r_address, r_user, r_pass, r_key)
        
        interpreters[interp_id] = interp
        interp_id += 1

        c.Write([]byte(strconv.Itoa(interp_id-1) + "\n"))

    // List all of the supported languages
    } else if command=="langs"{
        for k,_ := range(langs){
            c.Write([]byte(k + "\n"))
        }
        
    // Send a request and wait for the response
    // request id command
    } else if command=="request" {  
        iid_s, content := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        
        i.Request(string(content))
        i.Read()
        c.Write([]byte(i.Stdout))
    
    // Send a request
    } else if command=="send" { 
        iid_s, content := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        i.Write(string(content))
        c.Write([]byte("ok\n"))
    
    // Stop a specific interpreter
    } else if command=="stop" { 
        iid_s, _ := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        i.Close()
        delete(interpreters,iid)
        c.Write([]byte("ok\n"))
    
    // Show details of interpreter
    } else if command=="show" { 
        iid_s, _ := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        i.Show()

    // Inquire if interpreter is ready
    } else if command=="ready" { 
        iid_s, _ := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        r := strconv.Itoa(int(i.Read()))
        c.Write([]byte(r))
    
    // see the output of interpreter
    } else if command=="out" { 
        iid_s, _ := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        c.Write([]byte(i.Stdout))
    
    // see the error message from interpreter
    } else if command=="err" { 
        iid_s, _ := nextArg(content, " ")
        iid, _ := strconv.Atoi(iid_s)
        i := interpreters[iid]
        c.Write([]byte(i.Stderr))
    
    // List all of the current interpreters
    } else if command=="list" {
        for n,i := range(interpreters){
            c.Write([]byte( strconv.Itoa(n) +" " + i.Status() + "\n" ))
        }
    
    // End fleet
    } else if command=="exit" {
        c.Write([]byte("ok\n"))
        os.Exit(0)

    // Echo the input
    } else if command=="echo" {
        c.Write([]byte(content))

    } else if command=="groups" {
        for k,_ := range(groups){
            c.Write([]byte( k + "\n" ))
        }

    } else if command=="group" {
    
        groupname, content := nextArg(content, " ")
        command, content = nextArg(content, " ")
        
        switch command {
            case "new":
                groups[groupname] = NewGroup()
                
            case "start":
                for _,v := range(groups[groupname].Hosts){
                    v.GetStatus()
                }
            
                lang, _ := nextArg(content, " ")
                interps := groups[groupname].Pool(lang, 1, 0.001)
                interpreters[interp_id] = interps[0]
                interp_id += 1
                c.Write([]byte(strconv.Itoa(interp_id-1) + "\n"))
            
            case "list":
                for k,v := range(groups[groupname].Hosts){
                    c.Write([]byte( k +"\t"+v.Username + "\n" ))
                }
                
            case "status":
                for k,v := range(groups[groupname].Hosts){
                    c.Write([]byte( k +"\t"+v.Username + "\n" ))
                    v.GetStatus()
                    s := v.showStatus()
                    c.Write([]byte( s ))
                    c.Write([]byte("\n"))
                }
                
            case "add":
                r_address, content := nextArg(content, " ")
                r_user, content := nextArg(content, " ")
                r_pass, content := nextArg(content, " ")
                groups[groupname].AddHost(r_address).Login(r_user, r_pass)
            
            case "save":
                filename, _ := nextArg(content, " ")
                groups[groupname].Save(filename)
                
            case "load":
                filename, _ := nextArg(content, " ")
                groups[groupname] = LoadGroup(filename)
        }
    }
}


func handleConnection(c net.Conn, magic_key string){
    defer c.Close()
    
    // make sure the magic key matches
    magicbuf := make([]byte,32);
    _,_ = c.Read(magicbuf)

    if string(magicbuf) != magic_key{
        c.Close()
        return
    }

    // read the size of the message
    sizebuf := make([]byte,12);
    _,_ = c.Read(sizebuf)
    
    size,err := strconv.ParseInt(strings.Trim(string(sizebuf), " "),10,64)
    check(err)
    
    // read the message
    buffer := make([]byte, size)
    _,_ = c.Read(buffer)
    
    // execute the commands
    doCommand(buffer, c)
}


func main(){

    args := ParseArgs()
    //~ flag.Parse()

    if _,ok := args.Flags["s"];ok { // SERVER
        interpreters = make(map[int]*Interp)
        interp_id = 1
        groups = make(map[string]*Group)
        address := "localhost:8085"
        rand.Seed(time.Now().UTC().UnixNano())
        magic := fmt.Sprintf("%032d",rand.Int())
        
        ioutil.WriteFile("fleet.dat", []byte(address + "\n" + magic + "\n"), 066)
        
        ln, err := net.Listen("tcp", address)
        check(err)
        
        for {
            conn, err := ln.Accept()
            check(err)
            go handleConnection(conn, magic)
        }

    } else{          // CLIENT

        d1, err := ioutil.ReadFile("fleet.dat")
        d2 := strings.Split(string(d1), "\n")

        address := d2[0]
        magic   := d2[1]

        conn, err := net.Dial("tcp", address)
        check(err)
        
        total_message := strings.Join(args.Words, " ")
        
        // if -r flag read from standard in
        if _,ok := args.Flags["r"]; ok {
            stdin_message, err := ioutil.ReadAll(os.Stdin)
            check(err)
            total_message = total_message + " " + string(stdin_message)
        }
        
        fmt.Fprintf(conn, "%s%12d%s", magic, len(total_message), total_message)
        
        buf := make( []byte,0 )
        tmp := make( []byte,256 )
        
        for{
            n ,err := conn.Read(tmp)
            if err != nil{
                if err != io.EOF{
                    fmt.Println("read error", err)
                }
                break
            }
            buf = append(buf,tmp[:n]...)
        }
        
        fmt.Printf(string(buf))
    }
}

