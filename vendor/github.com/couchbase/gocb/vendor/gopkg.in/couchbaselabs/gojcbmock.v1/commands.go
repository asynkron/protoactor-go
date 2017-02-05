package gojcbmock

import (
	"encoding/json"
	"log"
	"strings"
)

type CmdCode string

const (
	CFailover        CmdCode = "FAILOVER"
	CRespawn                 = "RESPAWN"
	CHiccup                  = "HICCUP"
	CTruncate                = "TRUNCATE"
	CMockinfo                = "MOCKINF"
	CPersist                 = "PERSIST"
	CCache                   = "CACHE"
	CUnpersist               = "UNPERSIST"
	CUncache                 = "UNCACHE"
	CEndure                  = "ENDURE"
	CPurge                   = "PURGE"
	CKeyinfo                 = "KEYINFP"
	CGetMcPorts              = "GET_MCPORTS"
	CSetCCCP                 = "SET_CCCP"
	CRegenVBCoords           = "REGEN_VBCOORDS"
	CResetQueryState         = "RESET_QUERYSTATE"
)

type command struct {
	Code CmdCode
	Body map[string]interface{}
}

func (c command) Encode() (encoded []byte) {
	payload := make(map[string]interface{})
	payload["command"] = c.Code
	if c.Body != nil {
		payload["payload"] = c.Body
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic("Received invalid command for marshal")
	}
	return
}

func (c command) Set(key string, value interface{}) {
	c.Body[key] = value
}

type Command interface {
	Encode() []byte
	Set(key string, value interface{})
}

type Response struct {
	Payload map[string]interface{}
}

func (r *Response) Success() bool {
	s, exists := r.Payload["status"]
	if !exists {
		log.Print("Warning: status field not found!")
		return false
	}

	b, castok := s.(string)
	if !castok {
		log.Print("Bad type in 'status'")
		return false
	}
	return strings.ToLower(b)[0] == 'o'
}

func NewCommand(code CmdCode, body map[string]interface{}) Command {
	return command{Code: code, Body: body}
}
