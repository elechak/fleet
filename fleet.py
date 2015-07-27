import time

langs = {}

langs["lua"] = """lua -e '
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
"""


langs["bash"] = """bash -c '
while :
do
read -r -d "\x04" __C;
[[ ${#__C} = 0 ]] && exit;
eval "$__C";
echo -n -e "\x04";
echo -n -e "\x04" 1>&2;
done
'
"""


#~ langs["bash"] = """bash -c '
#~ which bash >>xxx/stuff.txt
#~ read -n 2 __C
#~ echo -e "$__C" >>xxx/stuff.txt
#~ echo -n -e "\x04";
#~ echo -n -e "\x04" 1>&2;
#~ done
#~ '
#~ """



langs["docker-bash"] = """ docker run --rm=true -i ubuntu /bin/bash -c '
while :
do
read -d "\x04" __C;
[[ ${#__C} = 0 ]] && exit;
eval "$__C";
echo -n -e "\x04";
echo -n -e "\x04" 1>&2;
done
'
"""




langs["nodejs"] = """nodejs -e '
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
"""

langs["python3"] = """python3 -c '
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
"""

langs["docker-python3"] = """ docker run --rm=true -i ubuntu /usr/bin/python3.4 -c '
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
"""




langs["perl"] = """perl -w -e '
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
"""

class Interp:  
    def __init__(self,lang="python3", host="", user="", password="", key="", ):
        import paramiko
        self.lang = lang
        self.host = host
        self.user = user
        self.write_time = 0
        
        self.stdout            = ""        
        self.stdout_buffer     = ""
        self.stdout_read_time  = 0
        
        self.stderr            = ""
        self.stderr_buffer     = ""
        self.stderr_read_time  = 0
        
        self.exit_status       = ""
        self.exit_read_time    = 0
        
        trans = paramiko.Transport((host,22))
        if password and key:
            trans.connect(username=user, password=password, pkey=paramiko.RSAKey.from_private_key_file(key))
        if password:
            trans.connect(username=user, password=password)
        elif key:
            trans.connect(username=user, pkey=paramiko.RSAKey.from_private_key_file(key))
            
        self.chan = trans.open_channel("session")
        
        print("----------------------")
        print(langs[lang])
        print("----------------------")
        
        self.chan.exec_command(langs[lang])


    def write(self,request):
        self.stdout_read_time = 0
        self.stdout=""
        self.stderr_read_time = 0
        self.stderr=""        
        self.chan.sendall(request + "\x04")
        self.write_time = time.time()


    def read(self):            
        if self.chan.recv_ready():
            self.stdout_buffer += self.chan.recv(4092).decode('ascii')
            if "\x04" in self.stdout_buffer:
                self.stdout_read_time = time.time()
                self.stdout, self.stdout_buffer = self.stdout_buffer.split("\x04",1)

        if self.chan.recv_stderr_ready():
            self.stderr_buffer += self.chan.recv_stderr(4092).decode('ascii')
            if "\x04" in self.stderr_buffer:
                self.stderr_read_time = time.time()
                self.stderr, self.stderr_buffer = self.stderr_buffer.split("\x04",1)

        if self.chan.exit_status_ready():
            self.exit_read_time = time.time()
            self.exit_status = self.chan.recv_exit_status()

        if self.stdout_read_time and self.stderr_read_time : 
            return max(self.stdout_read_time, self.stderr_read_time)
        
        return self.exit_read_time
        
    def show(self):
        print("Host: ", self.host)
        print("Lang: ", self.lang)
        print("User: ", self.user)
        print("Stdout: <%f> %s" % ( self.stdout_read_time, self.stdout))
        print("Stderr: <%f> %s" % (self.stderr_read_time, self.stderr))
        print("Exit: ", self.exit_status)
        print("-----------------------------")

    def time(self):
        return self.read() - self.write_time

    def request(self, req, delay=0.1):
        self.write(req)
        self.wait(delay)
        return self.stdout, self.stderr, self.exit_status

    def wait(self, delay=0.1):
        while not self.read(): time.sleep(delay)


def write(interps, data):
    for i in interps: i.write(data)
    
def wait(interps, delay=0.1):
    while not all(i.read() for i in interps): time.sleep(delay)

def request(interps, data, delay=0.1):
    write(interps, data)
    wait(interps, delay)

def map(interps, tasks, action, delay=0.1):
    for i in interps: i.stdout=""
    while tasks:
        for i in interps:
            if i.read():
                action(i,tasks.pop(0))
    wait(interps, delay)


#~ i = Interp("python3", "lion", "erik", "erik", "")
#~ j = i.request('print("python3", end="")')
#~ print(j)

#~ i = Interp("perl", "lion", "erik", "erik", "")
#~ j = i.request('print "perl"')
#~ print(j)

#~ i = Interp("nodejs", "lion", "erik", "erik", "")
#~ j = i.request('console.log("nodejs")')
#~ print(j)

i = Interp("bash", "lion", "erik", "erik", "")
j = i.request('echo -n bash')
print(j)

#~ i = Interp("lua", "lion", "erik", "erik", "")
#~ j = i.request('io.write("lua")')
#~ print(j)

#~ i = Interp("bash", "lion", "erik", "erik", "")
#~ j = i.request('A')
#~ print(j)
    