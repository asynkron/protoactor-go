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

    public sealed class SuspendMailbox : SystemMessage
    {
    }

    public sealed class ResumeMailbox : SystemMessage
    {
    }

    public sealed class Watch : SystemMessage
    {
        public Watch(PID watcher)
        {
            Watcher = watcher;
        }

        public PID Watcher { get; }
    }

    public sealed class Stop : SystemMessage
    {
        public static readonly Stop Instance = new Stop();
    }

    public sealed class Stopping : AutoReceiveMessage
    {
        public static readonly Stopping Instance = new Stopping();
    }

    public sealed class Started : AutoReceiveMessage
    {
        public static readonly Started Instance = new Started();
    }

    public sealed class Stopped : AutoReceiveMessage
    {
        public static readonly Stopped Instance = new Stopped();
    }
}