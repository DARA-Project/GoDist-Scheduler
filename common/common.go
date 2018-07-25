package common

import (
	"dara"
	"fmt"
	"reflect"
	"strings"
)


func ScheduleString(s *dara.Schedule) string {
	var output string
	for i := range *s {
		output += EventString(&(*s)[i])
	}
	return output
}

func RoutineInfoString(ri *dara.RoutineInfo) string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",dara.GStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}


//Printing Functions
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
	syscallstring += fmt.Sprintf("%d",(*gs).SyscallNum) // TODO use the func name lookup insted
	syscallstring += "("
	for i := 0;i<(*gs).NumArgs;i++ {
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

func EventString(e *dara.Event) string{
	return fmt.Sprintf("Event [Type: %s, P: %d, G: %s, Epoch: %d, LE: %s, SyscallInfo: %s,M: %s]",
		fmt.Sprintf("%d",(*e).Type),	//TODO replace with an array to string
		(*e).P,
		RoutineInfoString(&((*e).G)),
		(*e).Epoch,
		LogEntryString(&((*e).LE)),
		GeneralSyscallString(&((*e)).SyscallInfo),
		MessageString(&((*e)).Msg),
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
