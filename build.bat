cmd.exe /c buildprotos.bat ./actor
cmd.exe /c buildprotos.bat ./remoting
cmd.exe /c buildprotos.bat ./examples/chat/messages
cmd.exe /c buildprotos.bat ./examples/distributedchannels/messages
cmd.exe /c buildprotos.bat ./examples/remotebenchmark/messages


go build ./queue
go build ./actor
go build ./remoting
go build ./examples/becomeunbecome
go build ./examples/chat/server
go build ./examples/chat/client
go build ./examples/distributedchannels/node1
go build ./examples/distributedchannels/node2
go build ./examples/helloworld
go build ./examples/lifecycleevents
go build ./examples/remotebenchmark/node1
go build ./examples/remotebenchmark/node2
go build ./examples/supervison

rm *.exe