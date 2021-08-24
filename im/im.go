package im

import (
	"go_im/im/api"
	"go_im/im/client"
	"go_im/im/conn"
	"go_im/im/dao"
	"go_im/im/group"
)

func Run() {

	api.SetImpl(newApiRouter())
	group.Manager = NewGroupManager()
	client.Manager = newClientManager()

	dao.Init()
	wsServer := conn.NewWsServer(nil)
	wsServer.Handler(func(conn conn.Connection) {
		client.Manager.ClientConnected(conn)
	})
	wsServer.Run()
}
