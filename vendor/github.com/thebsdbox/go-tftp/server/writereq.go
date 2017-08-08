package server

import (
	"errors"
	"net"

	log "github.com/Sirupsen/logrus"

	pkt "github.com/whyrusleeping/go-tftp/packet"
)

// HandleWriteRequest makes a UDP connection back to the client
// and completes a TFTP Write request with them
func (s *Server) HandleWriteReq(wrq *pkt.ReqPacket, addr *net.UDPAddr) error {
	log.Infof("Write Request: %s", wrq.Filename)

	// 'Our' Address
	listaddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return err
	}

	// Connection directly to their open port
	con, err := net.DialUDP("udp", listaddr, addr)
	if err != nil {
		return err
	}

	if s.ReadOnly {
		errPkt := pkt.ErrorPacket{}
		errPkt.Value = "writing disallowed"
		errPkt.Code = pkt.TFTPErrAccessViolation
		_, err := con.Write(errPkt.Bytes())
		return err
	}

	// Lol, security? What security?
	fi, err := s.WriteFunc(s.servdir + "/" + wrq.Filename)
	if err != nil {
		return err
	}

	// Send ACK(0)
	ackPkt := pkt.NewAck(0)
	_, err = con.Write(ackPkt.Bytes())
	if err != nil {
		return err
	}

	curblk := uint16(1)
	buf := make([]byte, TftpMaxPacketSize)
	for {
		n, _, err := con.ReadFromUDP(buf)
		if err != nil {
			return err
		}

		idata, err := pkt.ParsePacket(buf[:n])
		if err != nil {
			return err
		}

		data, ok := idata.(*pkt.DataPacket)
		if !ok {
			return ErrUnexpectedPacket
		}

		if data.BlockNum == curblk-1 {
			// They didnt get our ack... lets send it again
			ackPkt := pkt.NewAck(curblk - 1)
			_, err = con.Write(ackPkt.Bytes())
			if err != nil {
				return err
			}

			continue
		} else if data.BlockNum != curblk {
			return errors.New("Received unexpected blocknum... stopping transfer.")
		}

		_, err = fi.Write(data.Data)
		if err != nil {
			return err
		}

		ackPkt := pkt.NewAck(curblk)
		_, err = con.Write(ackPkt.Bytes())
		if err != nil {
			return err
		}

		if len(data.Data) < 512 {
			return nil
		}

		curblk++
	}
}
