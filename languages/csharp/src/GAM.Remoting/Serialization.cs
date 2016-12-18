using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace GAM.Remoting
{
    public static class Serialization
    {
        private static Dictionary<string, MessageParser> _typeLookup;
        public static void Init()
        {
            var fileDescriptors =
                (from asm in AppDomain.CurrentDomain.GetAssemblies()
                    from type in asm.GetTypes()
                    where type.Name == "ProtosReflection"
                    let prop = type.GetProperty("Descriptor")
                    select (FileDescriptor) prop.GetValue(null)).ToArray();

            _typeLookup = new Dictionary<string, MessageParser>();
            foreach (var fd in fileDescriptors)
            {
                foreach (var msg in fd.MessageTypes)
                {
                    var name = fd.Package + "." + msg.Name;
                    var parser = (MessageParser)msg.ClrType.GetProperty("Parser").GetValue(null);
                    _typeLookup.Add(name, parser);
                }
            }
        }

        public static byte[] Serialize(IMessage message)
        {
            return message.ToByteArray();
        }

        public static object Deserialize(string typeName, ByteString bytes)
        {
            var parser = _typeLookup[typeName];
            var o = parser.ParseFrom(bytes);
            return o;
        }
    }
}
