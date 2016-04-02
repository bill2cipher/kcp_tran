// Transfer uses kcp to transport data betwween kcp client and server.
// By default, transfer split data into specified size trunck, and append
// md5 checksum after data. The recv side read data from network, check
// received data, append it to the output buffer(file). If there's anything
// wrong with the received data, transfer will ask the other side to resend
// the failed part. After receiving all the data, transfer will recheck all
// the recevied data with md5 again to make sure data completeness.
package transfer