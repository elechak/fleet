
package fleet

import (
    "golang.org/x/crypto/ssh"
    "fmt"
    "bytes"
    "time"
    "strings"
    "io"
    "io/ioutil"
)


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


type Interp struct{
    lang string
    host string
    user string
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

func check(err error){
    if err != nil{
        panic(err)
    }
}


func NewInterp(lang, host, user, password, key string) *Interp{  

    i := new(Interp)
    i.lang = lang
    i.host = host
    i.user = user
    
    am := make([]ssh.AuthMethod, 0)
    
    config := &ssh.ClientConfig{User: user}
    
    if len(key) > 0{
        data,err := ioutil.ReadFile(key)
            check(err)
            
        k, err := ssh.ParsePrivateKey(data)
            check(err)
        
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


// POOL FUNCTIONS

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




