using System.Collections.Generic;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace GAM.Remoting
{
    public static class Serialization
    {
        private static readonly Dictionary<string, MessageParser> TypeLookup = new Dictionary<string, MessageParser>();
        static Serialization()
        {
            RegisterFileDescriptor(GAM.ProtosReflection.Descriptor);
            RegisterFileDescriptor(GAM.Remoting.ProtosReflection.Descriptor);
        }

        public static void RegisterFileDescriptor(FileDescriptor fd)
        {
            foreach (var msg in fd.MessageTypes)
            {
                var name = fd.Package + "." + msg.Name;
                TypeLookup.Add(name, msg.Parser);
            }
        }

        public static ByteString Serialize(IMessage message)
        {
            return message.ToByteString();
        }

        public static object Deserialize(string typeName, ByteString bytes)
        {
            var parser = TypeLookup[typeName];
            var o = parser.ParseFrom(bytes);
            return o;
        }
    }
}
