package gocbcore

import (
	"strings"
	"time"
)

type AuthClient interface {
	Address() string

	ExecSaslListMechs(deadline time.Time) ([]string, error)
	ExecSaslAuth(k, v []byte, deadline time.Time) ([]byte, error)
	ExecSaslStep(k, v []byte, deadline time.Time) ([]byte, error)
	ExecSelectBucket(b []byte, deadline time.Time) error
}

type authClient struct {
	pipeline *memdPipeline
}

func (client *authClient) Address() string {
	return client.pipeline.Address()
}

func (client *authClient) doBasicOp(cmd CommandCode, k, v []byte, deadline time.Time) ([]byte, error) {
	resp, err := client.pipeline.ExecuteRequest(&memdQRequest{
		memdRequest: memdRequest{
			Magic:  ReqMagic,
			Opcode: cmd,
			Key:    k,
			Value:  v,
		},
	}, deadline)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

func (client *authClient) ExecSaslListMechs(deadline time.Time) ([]string, error) {
	bytes, err := client.doBasicOp(CmdSASLListMechs, nil, nil, deadline)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(bytes), " "), nil
}

func (client *authClient) ExecSaslAuth(k, v []byte, deadline time.Time) ([]byte, error) {
	logDebugf("Performing SASL authentication. %s %v", k, v)
	return client.doBasicOp(CmdSASLAuth, k, v, deadline)
}

func (client *authClient) ExecSaslStep(k, v []byte, deadline time.Time) ([]byte, error) {
	return client.doBasicOp(CmdSASLStep, k, v, deadline)
}

func (client *authClient) ExecSelectBucket(b []byte, deadline time.Time) error {
	_, err := client.doBasicOp(CmdSelectBucket, nil, b, deadline)
	return err
}
