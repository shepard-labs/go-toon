package toon

type ResourceLimits struct {
	MaxDepth       int
	MaxNodes       int
	MaxBytes       int64
	MaxStringBytes int
	MaxArrayLength int
}

type LimitCounter struct {
	Limits ResourceLimits
	nodes  int
}

func NewLimitCounter(limits ResourceLimits) *LimitCounter {
	return &LimitCounter{Limits: limits}
}

func (c *LimitCounter) CheckInputBytes(n int64) error {
	if c != nil && c.Limits.MaxBytes > 0 && n > c.Limits.MaxBytes {
		return Errorf(ErrResourceLimit, "input byte limit exceeded: %d > %d", n, c.Limits.MaxBytes)
	}
	return nil
}

func (c *LimitCounter) CheckStringBytes(n int) error {
	if c != nil && c.Limits.MaxStringBytes > 0 && n > c.Limits.MaxStringBytes {
		return Errorf(ErrResourceLimit, "string byte limit exceeded: %d > %d", n, c.Limits.MaxStringBytes)
	}
	return nil
}

func (c *LimitCounter) AddNode() error {
	if c == nil {
		return nil
	}
	c.nodes++
	if c.Limits.MaxNodes > 0 && c.nodes > c.Limits.MaxNodes {
		return Errorf(ErrResourceLimit, "node limit exceeded: %d > %d", c.nodes, c.Limits.MaxNodes)
	}
	return nil
}

func (c *LimitCounter) CheckDepth(depth int) error {
	if c != nil && c.Limits.MaxDepth > 0 && depth > c.Limits.MaxDepth {
		return Errorf(ErrResourceLimit, "depth limit exceeded: %d > %d", depth, c.Limits.MaxDepth)
	}
	return nil
}

func (c *LimitCounter) CheckArrayLength(n int) error {
	if c != nil && c.Limits.MaxArrayLength > 0 && n > c.Limits.MaxArrayLength {
		return Errorf(ErrResourceLimit, "array length limit exceeded: %d > %d", n, c.Limits.MaxArrayLength)
	}
	return nil
}
