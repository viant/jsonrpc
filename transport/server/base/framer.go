package base

// FrameMessage is a function type that allows wrapping of the message before sending it to the client
type FrameMessage func(data []byte) []byte
