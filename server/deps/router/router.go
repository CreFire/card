package router

import (
	"card/server/deps/msg"
	"card/server/deps/netmgr"
	rpcmgr "card/server/deps/rpc_mgr"

	"google.golang.org/protobuf/proto"
)

type S2SHandlerFunc func(msgque netmgr.IMsgQue, msg *msg.Message)
type C2SHandlerFunc func(msgque netmgr.IMsgQue, msg *msg.Message) (int32, proto.Message)

type Router struct {
	Chandlers map[int32]C2SHandlerFunc
	Shandlers map[int32]S2SHandlerFunc
	rpc       *rpcmgr.RpcMgr
}

func NewRouter(rpc *rpcmgr.RpcMgr) *Router {
	return &Router{
		Chandlers: make(map[int32]C2SHandlerFunc),
		Shandlers: make(map[int32]S2SHandlerFunc),
		rpc:       rpc,
	}
}

func (r *Router) RpcRegister(msgId int32, handler rpcmgr.RpcRequestHandler) {
	r.rpc.RpcRegister(msgId, handler)
}

func (r *Router) SSRegister(msgId int32, handler S2SHandlerFunc) {
	if _, ok := r.Shandlers[msgId]; ok {
		xlog.Warnf("router register repeated msgId:%v", msgId)
	}
	r.Shandlers[msgId] = handler
}

func (r *Router) CSRegister(msgId int32, handler C2SHandlerFunc) {
	if _, ok := r.Chandlers[msgId]; ok {
		xlog.Warnf("router register repeated msgId:%v", msgId)
	}
	r.Chandlers[msgId] = handler
}

func (r *Router) GetHandler(msgId int32) (handler C2SHandlerFunc, ok bool) {
	handler, ok = r.Chandlers[msgId]
	return handler, ok
}

func (r *Router) Dispatch(msgque netmgr.IMsgQue, message *msg.Message) bool {
	msgId := int32(message.MsgId())
	handler, ok := r.Chandlers[msgId]
	if !ok {
		return false
	}
	handler(msgque, message)
	return true
}
