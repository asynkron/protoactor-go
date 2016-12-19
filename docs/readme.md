# Cross platform actors

Introducing cross platform actor support between Go and C#.

Can I use this?
The Go implementation is still in beta, there are users using GAM for Go in production already.
But be aware that the API might change over time until 1.0.

The C# implementation is fresh out of the bakery, thus unstable and in alpha version.

## Design principles:

**Minimalistic API** -
The API should be small and easy to use.
Avoid enterprisey JVM like containers and configurations.

**Build on existing technologies** - There are already a lot of great tech for e.g. networking and clustering, build on those.
e.g. gRPC streams for networking, Consul.IO for clustering.

**Pass data, not objects** - Serialization is an explicit concern, don't try to hide it.
Protobuf all the way.

**Be fast** - Do not trade performance for magic API trickery.

Ultra fast remoting, GAM currently manages to pass over **two million messages per second** between nodes using only two actors, while still preserving message order!
This is six times more the new super advanced UDP based Artery transport for Scala Akka, and 30 times faster than Akka.NET.

## Sourcecode

The C# implementation can be found here [https://github.com/AsynkronIT/gam/tree/dev/languages/csharp](https://github.com/AsynkronIT/gam/tree/dev/languages/csharp)

And the Go implementation here [https://github.com/AsynkronIT/gam](https://github.com/AsynkronIT/gam)

## History

As the creator of the Akka.NET project, I have come to some distinct conclusions while being involved in that project.
In Akka.NET we created our own thread pool, our own networking layer, our own serialization support, our own configuration support etc. etc.
This was all fun and challenging, it is however now my firm opinion that this is the wrong way to go about things.

**If possible, software should be composed, not built**, only add code to glue existing pieces together.
This yields a much better time to market, and allows us to focus on solving the actual problem at hand, in this case concurrency and distributed programming.

GAM builds on existing technologies, Protobuf for serialization, gRPC streams for network transport.
This ensures cross platform compatibility, network protocol version tolerance and battle proven stability.

Another extremely important factor here is business agility and having an exit strategy.
By being cross platform, your organization is no longer tied into a specific platform, if you are migrating from .NET to Go, 
This can be done while still allowing actor based services to communicate between platforms.

Reinvent by not reinventing.

//Roger
