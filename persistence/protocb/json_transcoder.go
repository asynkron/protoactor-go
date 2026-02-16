package protocb

import "encoding/json"

type transcoder struct{}

func (t transcoder) Decode(bytes []byte, flags uint32, out any) error {
	err := json.Unmarshal(bytes, &out)
	if err != nil {
		return err
	}
	return nil
}

func (t transcoder) Encode(value any) ([]byte, uint32, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, 0, err
	}
	return bytes, 0, nil
}
