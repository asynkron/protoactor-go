namespace GAM
{
    public class LocalActorRef : ActorRef
    {
        public IMailbox Mailbox { get;  }
        public LocalActorRef(IMailbox mailbox)
        {
            Mailbox = mailbox;
        }

        public override void Tell(object message)
        {
            Mailbox.PostUserMessage(message);
        }
    }
}