package common

import (
	"dara"
	"fmt"
)

func PrintRoutineInfo(ri *dara.RoutineInfo) string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",dara.GStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}

func PrintEvent(e *dara.Event) string {
		return fmt.Sprintf("[ProcID %d, %s]\n",(*e).ProcID,PrintRoutineInfo(&(*e).Routine))
}

func PrintSchedule(s *dara.Schedule) string{
	var output string
	for i := range *s {
		output += PrintEvent(&(*s)[i])
	}
	return output
}

