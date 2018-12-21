package main

const code = `
using System;
using System.Threading.Tasks;
using Google.Protobuf;
using Proto;
using Proto.Cluster;
using Proto.Remote;

namespace {{.CsNamespace}}
{
    public static class Grains
    {
		{{ range $service := .Services}}	
        internal static Func<I{{ $service.Name }}> _{{$service.Name}}Factory;

        public static void {{ $service.Name }}Factory(Func<I{{$service.Name}}> factory) 
        {
            _{{$service.Name}}Factory = factory;
            Remote.RegisterKnownKind("{{ $service.Name }}", Actor.PropsFromProducer(() => new {{ $service.Name }}Actor()));
        } 

        public static {{ $service.Name }}Client {{$service.Name}}(string id) => new {{$service.Name}}Client(id);
		{{ end }}	
    }

	{{ range $service := .Services}}	
    public interface I{{ $service.Name }}
    {
		{{ range $method := $service.Methods}}
        Task<{{$method.Output.Name}}> {{$method.Name}}({{$method.Input.Name}} request);
		{{ end }}
    }

    public class {{$service.Name}}Client
    {
        private readonly string _id;

        public {{$service.Name}}Client(string id)
        {
            _id = id;
        }

		{{ range $method := $service.Methods}}
        public async Task< {{ $method.Output.Name }}> {{ $method.Name }}( {{ $method.Input.Name }} request)
        {
            var pid = await Cluster.GetAsync(_id, "{{ $service.Name }}");
            var gr = new GrainRequest
            {
                Method = "{{ $method.Name }}",
                MessageData = request.ToByteString()
            };
            var res = await RootContext.Empty.RequestAsync<object>(pid, gr);
            if (res is GrainResponse grainResponse)
            {
                return {{ $method.Output.Name }}.Parser.ParseFrom(grainResponse.MessageData);
            }
            if (res is GrainErrorResponse grainErrorResponse)
            {
                throw new Exception(grainErrorResponse.Err);
            }
            throw new NotSupportedException();
        }
		{{ end }}
    }

    public class {{$service.Name}}Actor : IActor
    {
        private I{{$service.Name}} _inner;

        public async Task ReceiveAsync(IContext context)
        {
            switch (context.Message)
            {
                case Started _:
                {
                    _inner = Grains._{{$service.Name}}Factory();
                    break;
                }
                case GrainRequest request:
                {
                    switch (request.Method)
                    {
						{{ range $method := $service.Methods}}
                        case "SayHello":
                        {
                            var r = {{$method.Input.Name}}.Parser.ParseFrom(request.MessageData);
                            try
                            {
                                var res = await _inner.{{$method.Name}}(r);
                                var grainResponse = new GrainResponse
                                {
                                    MessageData = res.ToByteString(),
                                };
                                context.Respond(grainResponse);
                            }
                            catch (Exception x)
                            {
                                var grainErrorResponse = new GrainErrorResponse
                                {
                                    Err = x.ToString()
                                };
                                context.Respond(grainErrorResponse);
                            }

                            break;
                        }
						{{ end }}
                    }

                    break;
                }
            }
        }
    }
	{{ end }}	
}

`
