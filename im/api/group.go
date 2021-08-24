package api

import (
	"errors"
	"go_im/im/client"
	"go_im/im/dao"
	"go_im/im/group"
	"go_im/im/message"
)

type GroupApi struct{}

func (m *GroupApi) CreateGroup(msg *RequestInfo, request *CreateGroupRequest) error {

	g, err := m.createGroup(request.Name, msg.Uid)
	if err != nil {
		return err
	}
	members, _ := group.Manager.GetMembers(g.Gid)
	c := ContactResponse{
		Friends: []*UserInfoResponse{},
		Groups: []*GroupResponse{{
			Group:   *g.Group,
			Members: members,
		}},
	}
	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(-1, ActionUserAddFriend, c))

	// create user chat by default
	uc, err := dao.ChatDao.NewUserChat(g.Cid, msg.Uid, g.Gid, dao.ChatTypeGroup)
	if err != nil {
		return err
	}
	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(-1, ActionUserNewChat, uc))

	// add invited member to group
	if len(request.Member) > 0 {
		nMsg := &RequestInfo{
			Uid: msg.Uid,
			Seq: -1,
		}
		nReq := &AddMemberRequest{
			Gid: g.Gid,
			Uid: request.Member,
		}
		err = m.AddGroupMember(nMsg, nReq)
		if err != nil {
			resp := message.NewMessage(-1, ActionFailed, "add member failed, "+err.Error())
			client.Manager.EnqueueMessage(msg.Uid, resp)
		}
	}

	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(msg.Seq, ActionSuccess, "create group success"))
	return nil
}

func (m *GroupApi) GetGroupMember(msg *RequestInfo, request *GetGroupMemberRequest) error {

	members, err := group.Manager.GetMembers(request.Gid)
	if err != nil {
		return err
	}

	ms := make([]*GroupMemberResponse, 0, len(members))
	for _, member := range members {
		ms = append(ms, &GroupMemberResponse{
			Uid:        member.Uid,
			Nickname:   "",
			RemarkName: member.Remark,
			Type:       member.Type,
			Online:     client.Manager.IsOnline(member.Uid),
		})
	}

	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(msg.Seq, ActionSuccess, ms))
	return nil
}

func (m *GroupApi) GetGroupInfo(msg *RequestInfo, request *GroupInfoRequest) error {

	var groups []*GroupResponse

	for _, gid := range request.Gid {
		g := group.Manager.GetGroup(gid)
		members, err := group.Manager.GetMembers(gid)
		if err != nil {
			return err
		}
		gr := GroupResponse{
			Group:   *g.Group,
			Members: members,
		}
		groups = append(groups, &gr)
	}

	resp := message.NewMessage(msg.Seq, ActionSuccess, groups)
	client.Manager.EnqueueMessage(msg.Uid, resp)
	return nil
}

func (m *GroupApi) RemoveMember(msg *RequestInfo, request *RemoveMemberRequest) error {

	for _, uid := range request.Uid {
		err := dao.GroupDao.RemoveMember(request.Gid, uid)
		if err != nil {
			return err
		}
		_ = group.Manager.RemoveMember(request.Gid, uid)
		notifyResp := message.NewMessage(-1, ActionGroupRemoveMember, "you have been removed from the group xxx by xxx")
		client.Manager.EnqueueMessage(uid, notifyResp)
	}

	resp := message.NewMessage(msg.Seq, ActionSuccess, "remove member success")

	client.Manager.EnqueueMessage(msg.Uid, resp)
	return nil
}

func (m *GroupApi) AddGroupMember(msg *RequestInfo, request *AddMemberRequest) error {

	g := group.Manager.GetGroup(request.Gid)

	members, err := m.addGroupMember(g.Gid, request.Uid...)
	if err != nil {
		return err
	}

	// notify group member update group
	groupNotify := GroupAddMemberResponse{
		Gid:     g.Gid,
		Members: members,
	}
	group.Manager.DispatchNotifyMessage(msg.Uid, g.Gid, message.NewMessage(-1, ActionGroupAddMember, groupNotify))

	for _, member := range members {

		// add group to member's contacts list
		_, e := dao.UserDao.AddContacts(member.Uid, g.Gid, dao.ContactsTypeGroup, "")
		if e != nil {
			_ = group.Manager.RemoveMember(request.Gid, member.Uid)
			continue
		}
		ms, err := group.Manager.GetMembers(request.Gid)
		if err != nil {
			return err
		}
		//notify update contacts list
		c := ContactResponse{
			Friends: []*UserInfoResponse{},
			Groups: []*GroupResponse{{
				Group:   *g.Group,
				Members: ms,
			}},
		}
		client.Manager.EnqueueMessage(member.Uid, message.NewMessage(-1, ActionUserAddFriend, c))

		// default add user chat
		chat, er := dao.ChatDao.NewUserChat(g.Cid, member.Uid, g.Gid, dao.ChatTypeGroup)
		if er != nil {
			continue
		}
		// member subscribe group message
		group.Manager.PutMember(g.Gid, member)

		// notify update chat list
		client.Manager.EnqueueMessage(member.Uid, message.NewMessage(-1, ActionUserNewChat, chat))
	}

	return nil
}

func (m *GroupApi) ExitGroup(msg *RequestInfo, request *ExitGroupRequest) error {

	err := group.Manager.RemoveMember(request.Gid, msg.Uid)
	if err != nil {
		return err
	}

	err = dao.GroupDao.RemoveMember(request.Gid, msg.Uid)
	if err != nil {
		return err
	}
	resp := message.NewMessage(msg.Seq, ActionSuccess, "exit group success")
	client.Manager.EnqueueMessage(msg.Uid, resp)
	return err
}

func (m *GroupApi) JoinGroup(msg *RequestInfo, request *JoinGroupRequest) error {

	g := group.Manager.GetGroup(request.Gid)

	if g == nil {
		return errors.New("group does not exist")
	}

	ms, err := m.addGroupMember(request.Gid, msg.Uid)
	if err != nil {
		return err
	}

	_, err = dao.UserDao.AddContacts(msg.Uid, g.Gid, dao.ContactsTypeGroup, "")

	members, err := group.Manager.GetMembers(request.Gid)
	if err != nil {
		return err
	}

	c := ContactResponse{
		Friends: []*UserInfoResponse{},
		Groups: []*GroupResponse{{
			Group:   *g.Group,
			Members: members,
		}},
	}
	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(-1, ActionUserAddFriend, c))

	chat, err := dao.ChatDao.NewUserChat(g.Cid, msg.Uid, g.Gid, dao.ChatTypeGroup)
	if err != nil {
		_ = dao.GroupDao.RemoveMember(request.Gid, msg.Uid)
		return err
	}
	group.Manager.PutMember(g.Gid, ms[0])
	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(-1, ActionUserNewChat, chat))
	client.Manager.EnqueueMessage(msg.Uid, message.NewMessage(msg.Seq, ActionSuccess, "join group success"))
	return nil
}

func (m *GroupApi) createGroup(name string, uid int64) (*group.Group, error) {

	gp, err := dao.GroupDao.CreateGroup(name, uid)
	if err != nil {
		return nil, err
	}
	// create group chat
	chat, err := dao.ChatDao.CreateChat(dao.ChatTypeGroup, gp.Gid)
	if err != nil {
		// TODO undo
		return nil, err
	}
	g := group.NewGroup(gp.Gid, gp, chat.Cid, []*dao.GroupMember{})

	owner, err := dao.GroupDao.AddMember(gp.Gid, dao.GroupMemberAdmin, uid)
	if err != nil {
		// TODO undo create group
		return nil, err
	}
	_, err = dao.UserDao.AddContacts(uid, gp.Gid, dao.ContactsTypeGroup, "")
	if err != nil {
		// TODO undo
		return nil, err
	}
	group.Manager.AddGroup(g)
	group.Manager.PutMember(g.Gid, owner[0])
	return g, nil
}

func (m *GroupApi) addGroupMember(gid int64, uid ...int64) ([]*dao.GroupMember, error) {

	g := group.Manager.GetGroup(gid)
	memberUid := make([]int64, 0, len(uid))
	for _, u := range uid {
		// member exist
		if !g.HasMember(u) {
			memberUid = append(memberUid, u)
		}
	}
	if len(memberUid) == 0 {
		return nil, errors.New("already added")
	}

	// TODO query user info and notify group members, optimize query time
	exist, err2 := dao.UserDao.HasUser(memberUid...)
	if err2 != nil {
		return nil, err2
	}
	if !exist {
		return nil, errors.New("user does not exist")
	}

	members, err := dao.GroupDao.AddMember(gid, dao.GroupMemberUser, memberUid...)
	if err != nil {
		return nil, err
	}
	return members, nil
}
