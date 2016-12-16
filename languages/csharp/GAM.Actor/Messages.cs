// //-----------------------------------------------------------------------
// // <copyright file="Messages.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

namespace GAM
{
    public abstract class SystemMessage
    {
    }

    public sealed class SuspendMailbox : SystemMessage
    {
    }

    public sealed class ResumeMailbox : SystemMessage
    {
    }

    public sealed class Started { }
}