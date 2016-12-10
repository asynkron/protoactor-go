// //-----------------------------------------------------------------------
// // <copyright file="Delegates.cs" company="Asynkron HB">
// //     Copyright (C) 2015-2016 Asynkron HB All rights reserved
// // </copyright>
// //-----------------------------------------------------------------------

using System.Threading.Tasks;

namespace GAM.Actor
{
    public delegate Task Receive(Context ctx);
}