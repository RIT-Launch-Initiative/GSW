package db

import "github.com/AarC10/GSW-V2/proc"

type Handler interface {
	Insert(packet []proc.TelemetryPacket)
	CreateQuery(packet proc.TelemetryPacket) string
}
