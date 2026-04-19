// Package queue provides a generic message queue interface for application decoupling.
// This file contains built-in codec implementations.
package queue

import "encoding/json"

// JSONCodec 是基于 JSON 的默认编解码器实现。
type JSONCodec struct{}

// Encode 使用 JSON 编码将值转换为字节切片。
func (c *JSONCodec) Encode(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Decode 使用 JSON 解码将字节切片解析到目标值。
func (c *JSONCodec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// BytesCodec 是透传编解码器，直接操作 []byte，不做任何转换。
// 适用于消息体本身就是 []byte 的场景。
type BytesCodec struct{}

// Encode 直接返回字节切片。
// 如果 v 不是 []byte 类型，返回 ErrCodecFailed。
func (c *BytesCodec) Encode(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, ErrCodecFailed
	}
	return b, nil
}

// Decode 直接将数据复制到目标。
// v 应为 *[]byte 类型。
func (c *BytesCodec) Decode(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return ErrCodecFailed
	}
	*p = make([]byte, len(data))
	copy(*p, data)
	return nil
}
