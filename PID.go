package gam

func (pid PID) Tell(message interface{}){
    ref,_ := FromPID(pid)
    ref.Tell(message)
}