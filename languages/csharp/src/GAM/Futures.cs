using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace GAM
{
    public class FutureActor<T> : IActor
    {
        private readonly TaskCompletionSource<T> _tcs;

        public FutureActor(TaskCompletionSource<T> tcs)
        {
            _tcs = tcs;
        }

        public Task ReceiveAsync(IContext context)
        {
            var msg = context.Message;
            if (msg is T)
            {
                _tcs.TrySetResult((T) msg);
                context.Self.Stop();
            }
            
            return Task.FromResult(0);
        }
    }
}
