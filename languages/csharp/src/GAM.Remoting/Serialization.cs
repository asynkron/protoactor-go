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
        private static Dictionary<string, Type> _typeLookup;
        public static void Init()
        {
            var fileDescriptors =
                (from asm in AppDomain.CurrentDomain.GetAssemblies()
                    from type in asm.GetTypes()
                    where type.Name == "ProtosReflection"
                    let prop = type.GetProperty("Descriptor")
                    select (FileDescriptor) prop.GetValue(null)).ToArray();

            _typeLookup = new Dictionary<string, Type>();
            foreach (var fd in fileDescriptors)
            {
                foreach (var msg in fd.MessageTypes)
                {
                    var name = fd.Package + "." + msg.Name;
                    _typeLookup.Add(name, msg.ClrType);
                }
            }
        }

        public static byte[] Serialize(IMessage message)
        {
            return message.ToByteArray();
        }

        public static object Deserialize(string typeName, ByteString bytes)
        {
            //HACK, fix this..
            var type = _typeLookup[typeName];
            var parser = type.GetProperty("Parser").GetValue(null);
            var parseFrom = parser.GetType().GetMethod("ParseFrom",new Type[] {typeof(ByteString)});
            var o = parseFrom.Invoke(parser, new object[] {bytes});
            return o;
        }
    }
}
