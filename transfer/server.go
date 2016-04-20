package transfer

func Serve() error {
   server, err := kcp.Listen("0.0.0.0:8765", 1)
   if err != nil {
     return err
   }
   for {
     kdp, err := server.Accept()
     if err != nil {
       return err
     }
     NewEndPoint(1, kdp)     
   }
}
