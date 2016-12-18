// -----------------------------------------------------------------------
//  <copyright file="Messages.cs" company="Asynkron HB">
//      Copyright (C) 2015-2016 Asynkron HB All rights reserved
//  </copyright>
// -----------------------------------------------------------------------

namespace GAM
{
    public abstract class SystemMessage
    {
    }

    public abstract class AutoReceiveMessage
    {
    }

    public  sealed partial class SuspendMailbox : SystemMessage
    {
    }

    public  sealed partial class ResumeMailbox : SystemMessage
    {
    }

    public  sealed partial class Watch : SystemMessage
    {
        public Watch(PID watcher)
        {
            Watcher = watcher;
        }

      
    }

    public sealed partial class Stop : SystemMessage
    {
        public static readonly Stop Instance = new Stop();
    }

    public sealed partial class Stopping : AutoReceiveMessage
    {
        public static readonly Stopping Instance = new Stopping();
    }

    public sealed partial class Started : AutoReceiveMessage
    {
        public static readonly Started Instance = new Started();
    }

    public sealed partial class Stopped : AutoReceiveMessage
    {
        public static readonly Stopped Instance = new Stopped();
    }
}