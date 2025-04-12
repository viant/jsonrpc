package base

// Option represents option
type Option func(s *Session)

func WithFramer(framer FrameMessage) Option {
	return func(s *Session) {
		s.framer = framer
	}
}
