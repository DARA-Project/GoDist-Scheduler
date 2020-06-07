package common

import (
    "bytes"
	"dara"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
    "strconv"
)

func ReadSchedule(filename string) (*dara.Schedule, error) {
	var schedule dara.Schedule
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(&schedule)
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

func GoRoutineNameString(ProcID int, ri dara.RoutineInfo) string {
    func_name := string(bytes.Trim(ri.FuncInfo[:64], "\x00")[:])
    name := "PID-" + strconv.Itoa(ProcID) + "-G-" + strconv.Itoa(ri.Gid) + "-" + func_name
    return name
}

func ScheduleString(events *[]dara.Event) string {
	var output string
	for _, e := range *events {
		output += EventString(&e)
	}
	return output
}

func ConciseScheduleString(events *[]dara.Event) string {
    var output string
    for _, e := range *events {
        output += ConciseEventString(&e) + "\n"
    }
    return output
}

func RoutineInfoString(ri *dara.RoutineInfo) string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",dara.GStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}

func CoverageString(event *dara.CoverageEvent) string {
	var retString string
	var allStrings []string
	for key, val := range event.CoverageInfo {
		allStrings = append(allStrings, key + " - " + strconv.FormatUint(val, 10))
	}
	retString = strings.Join(allStrings, ",")
	return retString
}

func PropFailureString(event *dara.FailedPropertyEvent) string {
	return fmt.Sprintf("%s %v", event.Name, event.Context)
}

func ConciseRoutineInfoString(prefix string, ri dara.RoutineInfo) string {
    retString := prefix
    retString += " GoID " + fmt.Sprintf("%d",ri.Gid)
    retString += " Status " + dara.GStatusStrings[ri.Status]
    return retString
}

//Printing Functions

func EventString(e *dara.Event) string{
    typeString := EventTypeString((*e).Type)
    proc := (*e).P
    routineInfoString := RoutineInfoString(&((*e).G))
    epoch := (*e).Epoch
    LE := LogEntryString(&((*e).LE))
    syscallString := ""
    if typeString == "SYSCALL" {
        syscallString = GeneralSyscallString(&((*e)).SyscallInfo)
    }
    msgString := (MessageString(&((*e)).Msg))
	return fmt.Sprintf("Event [Type: %s, P: %d, G: %s, Epoch: %d, LE: %s, SyscallInfo: %s,M: %s]",
		typeString,	//TODO replace with an array to string
		proc,
		routineInfoString,
		epoch,
		LE,
		syscallString,
		msgString,
	)
}

func ConciseEventString(e *dara.Event) string {
    switch e.Type {
        case dara.LOG_EVENT :
            return "LOG"
        case dara.SYSCALL_EVENT :
            return ConciseSyscallString(e.SyscallInfo)
        case dara.SCHED_EVENT :
            return ConciseRoutineInfoString("SCHEDULE", e.G)
        case dara.SEND_EVENT :
            return "SEND"
        case dara.REC_EVENT :
            return "RECEIVE"
        case dara.INIT_EVENT :
            return "INIT"
        case dara.THREAD_EVENT :
            return ConciseRoutineInfoString("THREAD", e.G)
        case dara.END_EVENT :
            return "END"
        case dara.CRASH_EVENT :
            return "PANIC"
    }

    return ""
}

func EventTypeString(eventType int) string {
    switch(eventType) {
        case dara.LOG_EVENT :
            return "LOG"
        case dara.SYSCALL_EVENT :
            return "SYSCALL"
        case dara.SEND_EVENT :
            return "SEND"
        case dara.REC_EVENT :
            return "RECEIVE"
        case dara.SCHED_EVENT :
            return "SCHEDULE"
        case dara.INIT_EVENT :
            return "INIT"
        case dara.THREAD_EVENT :
            return "THREAD"
        case dara.END_EVENT :
            return "END"
        case dara.CRASH_EVENT :
            return "PANIC"
    }
    return ""
}

func GetEventTypes() []string {
    return []string{"LOG", "SYSCALL", "INIT", "END", "RECEIVE", "SCHEDULE", "THREAD", "PANIC"}
}

func SyscallNameString(syscallNum int) string {
    switch(syscallNum) {
	    case dara.DSYS_READ : return "READ"
	    case dara.DSYS_WRITE : return "WRITE"
	    case dara.DSYS_OPEN : return "OPEN"
	    case dara.DSYS_CLOSE : return "CLOSE"
	    case dara.DSYS_STAT : return "STAT"
	    case dara.DSYS_FSTAT : return "FSTAT"
	    case dara.DSYS_LSTAT : return "LSTAT"
	    case dara.DSYS_LSEEK : return "LSEEK"
	    case dara.DSYS_PREAD64 : return "PREAD64"
	    case dara.DSYS_PWRITE64 : return "PWRITE64"
	    case dara.DSYS_GETPAGESIZE : return "GETPAGESIZE"
	    case dara.DSYS_EXECUTABLE : return "EXECUTABLE"
	    case dara.DSYS_GETPID : return "GETPID"
	    case dara.DSYS_GETPPID : return "GETPPID"
	    case dara.DSYS_GETWD : return "GETWD"
	    case dara.DSYS_READDIR : return "READDIR"
	    case dara.DSYS_READDIRNAMES : return "READDIRNAMES"
	    case dara.DSYS_WAIT4 : return "WAIT4"
	    case dara.DSYS_KILL : return "KILL"
	    case dara.DSYS_GETUID : return "GETUID"
	    case dara.DSYS_GETEUID : return "GETEUID"
	    case dara.DSYS_GETGID : return "GETGID"
	    case dara.DSYS_GETEGID : return "GETEGID"
	    case dara.DSYS_GETGROUPS : return "GETGROUPS"
	    case dara.DSYS_EXIT : return "EXIT"
	    case dara.DSYS_RENAME : return "RENAME"
	    case dara.DSYS_TRUNCATE : return "TRUNCATE"
	    case dara.DSYS_UNLINK : return "UNLINK"
	    case dara.DSYS_RMDIR : return "RMDIR"
	    case dara.DSYS_LINK : return "LINK"
	    case dara.DSYS_SYMLINK : return "SYMLINK"
	    case dara.DSYS_PIPE2 : return "PIPE2"
	    case dara.DSYS_MKDIR : return "MKDIR"
	    case dara.DSYS_CHDIR : return "CHDIR"
	    case dara.DSYS_UNSETENV : return "UNSETENV"
	    case dara.DSYS_GETENV : return "GETENV"
	    case dara.DSYS_SETENV : return "SETENV"
	    case dara.DSYS_CLEARENV : return "CLEARENV"
	    case dara.DSYS_ENVIRON : return "ENVIRON"
	    case dara.DSYS_TIMENOW : return "TIMENOW"
	    case dara.DSYS_READLINK : return "READLINK"
	    case dara.DSYS_CHMOD : return "CHMOD"
	    case dara.DSYS_FCHMOD : return "FCHMOD"
	    case dara.DSYS_CHOWN : return "CHOWN"
	    case dara.DSYS_LCHOWN : return "LCHOWN"
	    case dara.DSYS_FCHOWN : return "FCHOWN"
	    case dara.DSYS_FTRUNCATE : return "FTRUNCATE"
	    case dara.DSYS_FSYNC : return "FSYNC"
	    case dara.DSYS_UTIMES : return "UTIMES"
	    case dara.DSYS_FCHDIR : return "FCHDIR"
	    case dara.DSYS_SETDEADLINE : return "SETDEADLINE"
	    case dara.DSYS_SETREADDEADLINE : return "SETREADDEADLINE"
	    case dara.DSYS_SETWRITEDEADLINE : return "SETWRITEDEADLINE"
	    case dara.DSYS_NET_READ : return "NET_READ"
	    case dara.DSYS_NET_WRITE : return "NET_WRITE"
	    case dara.DSYS_NET_CLOSE : return "NET_CLOSE"
	    case dara.DSYS_NET_SETDEADLINE : return "NET_SETDEADLINE"
	    case dara.DSYS_NET_SETREADDEADLINE : return "NET_SETREADDEADLINE"
	    case dara.DSYS_NET_SETWRITEDEADLINE : return "NET_SETWRITEDEADLINE"
	    case dara.DSYS_NET_SETREADBUFFER : return "NET_SETREADBUFFER"
	    case dara.DSYS_NET_SETWRITEBUFFER : return "NET_SETWRITEBUFFER"
	    case dara.DSYS_SOCKET : return "SOCKET"
	    case dara.DSYS_LISTEN_TCP : return "LISTEN_TCP"
        case dara.DSYS_SLEEP : return "SLEEP"
        case dara.MUX_LOCK : return "LOCK"
        case dara.MUX_UNLOCK : return "UNLOCK"
        case dara.WG_ADD : return "WG.ADD"
        case dara.WG_WAIT : return "WG.WAIT"
        case dara.CTX_DONE : return "CTX.DONE"
        case dara.CTX_CANCEL : return "CTX.CANCEL"
    }
    return ""
}

func EncEventString(e *dara.EncEvent) string{
	return fmt.Sprintf("EncEvent [Type: %s,P: %d,G: %s,Epoch: %d,ELE: %s,SyscallInfo: %s,EM: %s]",
		fmt.Sprintf("%d",(*e).Type),	//TODO replace with an array to string
		(*e).P,
		RoutineInfoString(&((*e).G)),
		EncLogEntryString(&((*e).ELE)),
		GeneralSyscallString(&((*e)).SyscallInfo),
		EncodedMessageString(&((*e)).EM),
	)
}

func EncLogEntryString(ele *dara.EncLogEntry) string {
	var strval string
	strval += fmt.Sprintf("EncLogEntry [LogID: %s,",string((*ele).LogID[:]))
	strval += "Vars ["
	for i:=0;i<(*ele).Length;i++{
		strval += EncNameValuePairString(&(*ele).Vars[i])
	}
	strval += "]]"
	return strval
}

func GeneralTypeString(gt *dara.GeneralType) string {
	switch (*gt).Type {
	case dara.INTEGER:
		return fmt.Sprintf("%d",(*gt).Integer)
	case dara.BOOL:
		return fmt.Sprintf("%t",(*gt).Bool)
	case dara.FLOAT:
		return fmt.Sprintf("%f",(*gt).Float)
	case dara.INTEGER64:
		return fmt.Sprintf("%d",(*gt).Integer64)
	case dara.STRING:
		return fmt.Sprintf("%s",(*gt).String)
	}
	return "Unsuported..."
}

func GeneralSyscallString(gs *dara.GeneralSyscall) string {
	var syscallstring string
	syscallstring += "func "
	syscallstring += SyscallNameString((*gs).SyscallNum) // TODO use the func name lookup insted
	syscallstring += "("
	for i := 0;i<(*gs).NumArgs;i++ {
        println(GeneralTypeString(&(gs.Args[i])))
		syscallstring += GeneralTypeString(&(*gs).Args[i])
		if i < (*gs).NumArgs -1 {
			syscallstring += ","
		}
	}
	syscallstring += ") ("
	for i := 0;i<(*gs).NumRets;i++ {
		syscallstring += GeneralTypeString(&(*gs).Rets[i])
		if i < (*gs).NumArgs -1 {
			syscallstring += ","
		}
	}
	syscallstring += ")"
	return syscallstring
}

func ConciseSyscallString(syscall dara.GeneralSyscall) string {
    retString := "SYSCALL "
    retString = retString + SyscallNameString(syscall.SyscallNum)
    return retString
}

func EncodedMessageString(em *dara.EncodedMessage) string {
	return string((*em).Body[:])
}

func EncNameValuePairString(encp *dara.EncNameValuePair) string {
	return fmt.Sprintf("EncNameValuePair [Name: %s,Value: %x,Type: %d,", //TODO use name table not integers
		string((*encp).VarName[:]),
		(*encp).Value,
		(*encp).Type[:],
	)
}

func LogEntryString(le *dara.LogEntry) string {
	var strval string
	strval += fmt.Sprintf("LogEntry [LogID: %s,",(*le).LogID)
	strval += "Vars ["
	for i:=0;i<len((*le).Vars);i++{
		strval += NameValuePairString(&(*le).Vars[i])
	}
	strval += "]]"
	return strval
}

//String representation of a name value pair
func NameValuePairString(nvp *dara.NameValuePair) string {
	return fmt.Sprintf("NVP [Name:%s,Value:%s,Type:%s]", (*nvp).VarName, ValueString((*nvp).Value), (*nvp).Type)
}

//returns the value of the Name value pair as a string
//TODO catch and print all possible reflected types
func ValueString(val interface{}) string {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%.2f", v.Float())
	case reflect.String:
		return fmt.Sprintf("\"%s\"", strings.Replace(fmt.Sprintf("%s", v.String()), "\n", " ", -1))
	default:
		return ""
	}
}

type Message struct {
	Body string
}

func MessageString(m *dara.Message) string {
	return (*m).Body
}
