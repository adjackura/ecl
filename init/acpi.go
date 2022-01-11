package main

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
)

var messageFuncs = map[string]func() error{
	"button/power": func() error {
		logger.Println("ACPI shutdown signal recieved")
		if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
			return err
		}
		return nil
	},
}

func waitForMessages(conn *genetlink.Conn) error {
	for {
		msgs, _, err := conn.Receive()
		if err != nil {
			return err
		}

		logger.Printf("Got messages: %q", msgs)

		for _, msg := range msgs {
			ad, err := netlink.NewAttributeDecoder(msg.Data)
			if err != nil {
				return err
			}

			for ad.Next() {
				logger.Printf("ACPI event: %q", ad.String())
				for m, f := range messageFuncs {
					if strings.HasPrefix(ad.String(), m) {
						if err := f(); err != nil {
							logger.Println("Error processing event:", err)
						}
					}
				}
			}
		}
	}
}

func runACPIListener() (err error) {
	conn, err := genetlink.Dial(nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	// https://github.com/torvalds/linux/blob/master/drivers/acpi/event.c#L77
	fam, err := conn.GetFamily("acpi_event")
	if errors.Is(err, os.ErrNotExist) {
		conn.Close()
		return err
	}

	var id uint32
	for _, g := range fam.Groups {
		// https: //github.com/torvalds/linux/blob/master/drivers/acpi/event.c#L79
		if g.Name == "acpi_mc_group" {
			id = g.ID
		}
	}

	if err := conn.JoinGroup(id); err != nil {
		return err
	}

	return waitForMessages(conn)
}
