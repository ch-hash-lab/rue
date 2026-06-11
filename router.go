package rue

// Router maps HTTP method + request path to a registered handler chain.
//
// Patterns are '/'-separated. Each pattern segment takes one of four forms:
//
//	literal      /users/all
//	capture      /users/:id    matches exactly one non-empty segment
//	prefixed     /file_:name   literal prefix plus capture of the remainder
//	trailing     /assets/*rest absorbs everything left of the path; must be
//	                           the final segment and directly follow a '/';
//	                           the captured value keeps its leading '/'
//
// Registration is write-side: all addRoute calls must finish before the
// router starts serving. Lookups (getValue) are read-only and safe for
// unlimited concurrent use once registration is done.
//
// Trailing-capture values are handed to handlers verbatim — a file-serving
// handler downstream owns its own ".." sanitization.
type Router struct {
	roots   map[string]*segNode             // method → segment-trie root
	statics map[string]map[string]staticHit // optional exact-match fast path; nil = disabled
	maxVars int                             // largest capture count of any registered pattern
}

// staticHit is one entry of the optional exact-match fast path.
type staticHit struct {
	chain   HandlersChain
	pattern string
}

// segNode is one path segment in the trie. A node owns up to four kinds of
// children, tried most-specific-first during a lookup: exact literals,
// prefixed captures (longest prefix first), one bare capture, and one
// trailing capture. The kinds are mutually exclusive per child, not per
// node — except a trailing capture, which cannot share a node with any
// other child kind (the position would be ambiguous).
type segNode struct {
	label      string     // literal text: whole segment (statics) or prefix (mixes)
	name       string     // capture name (mixes, capture, tail)
	statics    []*segNode // exact-literal children
	firstChars []byte     // firstChars[i] is the first byte of statics[i].label (0 for empty)
	mixes      []*segNode // prefixed-capture children, longest prefix first
	capture    *segNode   // bare ':name' child
	tail       *segNode   // '*name' child; terminal by construction
	chain      HandlersChain
	pattern    string // the registered pattern, set where chain is set
}

// Params holds path parameters
type Params []Param

// Param is a single path parameter
type Param struct {
	Key   string
	Value string
}

// Get returns the value of the first Param which key matches the given name
func (ps Params) Get(name string) (string, bool) {
	for _, p := range ps {
		if p.Key == name {
			return p.Value, true
		}
	}
	return "", false
}

// ByName returns the value of the first Param which key matches the given name
func (ps Params) ByName(name string) string {
	v, _ := ps.Get(name)
	return v
}

// newRouter creates a new Router
func newRouter() *Router {
	return &Router{roots: make(map[string]*segNode)}
}

// addRoute registers a new route. It panics on malformed patterns and on
// registrations that conflict with an existing route — both are developer
// errors that must surface at startup, never at request time.
func (r *Router) addRoute(method, path string, handlers HandlersChain) {
	if len(path) == 0 || path[0] != '/' {
		panic("rue: route pattern must start with '/'")
	}
	if method == "" {
		panic("rue: HTTP method is required")
	}
	if len(handlers) == 0 {
		panic("rue: route needs at least one handler")
	}

	root := r.roots[method]
	if root == nil {
		root = &segNode{}
		r.roots[method] = root
	}

	if vars := root.mount(path, handlers); vars > r.maxVars {
		r.maxVars = vars
	}
}

// getValue returns the handlers and params for a given path
func (r *Router) getValue(method, path string, params *Params) (HandlersChain, string, bool) {
	if r.statics != nil {
		if hit, ok := r.statics[method][path]; ok {
			return hit.chain, hit.pattern, true
		}
	}
	root := r.roots[method]
	if root == nil || len(path) == 0 || path[0] != '/' {
		return nil, "", false
	}
	return root.resolve(path, params, r.maxVars)
}

// mount registers pattern below n and returns the pattern's capture count.
func (n *segNode) mount(pattern string, chain HandlersChain) int {
	vars := 0
	cur := n
	start := 1
	for start <= len(pattern) {
		end := start
		for end < len(pattern) && pattern[end] != '/' {
			end++
		}
		seg := pattern[start:end]

		marker := -1
		for i := 0; i < len(seg); i++ {
			if seg[i] == ':' || seg[i] == '*' {
				marker = i
				break
			}
		}

		switch {
		case marker < 0:
			cur = cur.literalChild(seg, pattern)

		case seg[marker] == ':':
			name := seg[marker+1:]
			checkCaptureName(name, pattern)
			vars++
			if marker > 0 {
				cur = cur.prefixedChild(seg[:marker], name, pattern)
			} else {
				cur = cur.captureChild(name, pattern)
			}

		default: // '*'
			if marker != 0 {
				panic("rue: '*' capture must directly follow a '/' (pattern '" + pattern + "')")
			}
			if end != len(pattern) {
				panic("rue: '*' capture must be the final segment (pattern '" + pattern + "')")
			}
			checkCaptureName(seg[1:], pattern)
			vars++
			cur = cur.tailChild(seg[1:], pattern)
		}

		start = end + 1
	}

	if cur.chain != nil {
		panic("rue: duplicate registration for pattern '" + pattern + "'")
	}
	cur.chain = chain
	cur.pattern = pattern
	return vars
}

// checkCaptureName rejects empty and multi-marker capture names.
func checkCaptureName(name, pattern string) {
	if name == "" {
		panic("rue: ':' and '*' captures require a name (pattern '" + pattern + "')")
	}
	for i := 0; i < len(name); i++ {
		if name[i] == ':' || name[i] == '*' {
			panic("rue: a path segment may hold at most one ':' or '*' capture (pattern '" + pattern + "')")
		}
	}
}

func (n *segNode) literalChild(seg, pattern string) *segNode {
	var c byte
	if len(seg) > 0 {
		c = seg[0]
	}
	for i, fc := range n.firstChars {
		if fc == c && n.statics[i].label == seg {
			return n.statics[i]
		}
	}
	if n.tail != nil {
		panic("rue: pattern '" + pattern + "' conflicts with capture '*" + n.tail.name + "' registered at this position")
	}
	child := &segNode{label: seg}
	n.statics = append(n.statics, child)
	n.firstChars = append(n.firstChars, c)
	return child
}

func (n *segNode) prefixedChild(prefix, name, pattern string) *segNode {
	for _, m := range n.mixes {
		if m.label == prefix {
			if m.name != name {
				panic("rue: pattern '" + pattern + "' conflicts with capture ':" + m.name + "' already registered at this position")
			}
			return m
		}
	}
	if n.tail != nil {
		panic("rue: pattern '" + pattern + "' conflicts with capture '*" + n.tail.name + "' registered at this position")
	}
	child := &segNode{label: prefix, name: name}
	// keep the longest prefix first so the most specific form is tried first
	at := len(n.mixes)
	for i, m := range n.mixes {
		if len(m.label) < len(prefix) {
			at = i
			break
		}
	}
	n.mixes = append(n.mixes, nil)
	copy(n.mixes[at+1:], n.mixes[at:])
	n.mixes[at] = child
	return child
}

func (n *segNode) captureChild(name, pattern string) *segNode {
	if n.capture != nil {
		if n.capture.name != name {
			panic("rue: pattern '" + pattern + "' conflicts with capture ':" + n.capture.name + "' already registered at this position")
		}
		return n.capture
	}
	if n.tail != nil {
		panic("rue: pattern '" + pattern + "' conflicts with capture '*" + n.tail.name + "' registered at this position")
	}
	n.capture = &segNode{name: name}
	return n.capture
}

func (n *segNode) tailChild(name, pattern string) *segNode {
	if n.tail != nil {
		if n.tail.name != name {
			panic("rue: pattern '" + pattern + "' conflicts with capture '*" + n.tail.name + "' already registered at this position")
		}
		return n.tail
	}
	// A '*' capture absorbs every continuation, so it cannot share its
	// position with any other child kind — reject at startup rather than
	// let one of the two routes become unreachable.
	if len(n.statics) > 0 || len(n.mixes) > 0 || n.capture != nil {
		panic("rue: '*' capture (pattern '" + pattern + "') cannot share a position with other routes")
	}
	n.tail = &segNode{name: name}
	return n.tail
}

// matchFrame is a resume point for backtracking: retry node n at segment
// offset start, continuing with alternative kind stage at child index idx,
// after truncating captured params back to nvars.
type matchFrame struct {
	n     *segNode
	start int
	nvars int
	stage uint8
	idx   int
}

// Alternative kinds, in lookup priority order.
const (
	tryStatics uint8 = iota
	tryMixes
	tryCapture
	tryTail
)

// resolve walks path below n one segment at a time. At every node the
// alternatives are tried most-specific-first; when a branch dead-ends the
// walk resumes at the most recent node that still has untried alternatives,
// so a miss under a literal child falls through to a capture sibling.
// A resume point is recorded only when such alternatives exist, which keeps
// purely static lookups free of bookkeeping.
func (n *segNode) resolve(path string, params *Params, maxVars int) (HandlersChain, string, bool) {
	var room [16]matchFrame // usually enough; spills to the heap when exceeded
	frames := room[:0]

	cur := n
	start := 1
	stage := tryStatics
	idx := 0

	for {
		matched := false

		if start > len(path) {
			// every segment consumed — the chain, if any, lives here
			if cur.chain != nil {
				return cur.chain, cur.pattern, true
			}
		} else {
			end := start
			for end < len(path) && path[end] != '/' {
				end++
			}
			seg := path[start:end]

			if stage == tryStatics {
				var c byte
				if len(seg) > 0 {
					c = seg[0]
				}
				for i, fc := range cur.firstChars {
					if fc == c && cur.statics[i].label == seg {
						if len(cur.mixes) > 0 || cur.capture != nil || cur.tail != nil {
							frames = append(frames, matchFrame{cur, start, varCount(params), tryMixes, 0})
						}
						cur = cur.statics[i]
						start = end + 1
						stage, idx = tryStatics, 0
						matched = true
						break
					}
				}
				if !matched {
					stage, idx = tryMixes, 0
				}
			}

			if !matched && stage == tryMixes {
				for ; idx < len(cur.mixes); idx++ {
					m := cur.mixes[idx]
					if len(seg) > len(m.label) && seg[:len(m.label)] == m.label {
						nvars := varCount(params)
						if idx+1 < len(cur.mixes) || cur.capture != nil || cur.tail != nil {
							frames = append(frames, matchFrame{cur, start, nvars, tryMixes, idx + 1})
						}
						pushVar(params, m.name, seg[len(m.label):], maxVars)
						cur = m
						start = end + 1
						stage, idx = tryStatics, 0
						matched = true
						break
					}
				}
				if !matched {
					stage, idx = tryCapture, 0
				}
			}

			if !matched && stage == tryCapture {
				if cur.capture != nil && len(seg) > 0 {
					if cur.tail != nil {
						frames = append(frames, matchFrame{cur, start, varCount(params), tryTail, 0})
					}
					pushVar(params, cur.capture.name, seg, maxVars)
					cur = cur.capture
					start = end + 1
					stage, idx = tryStatics, 0
					matched = true
				} else {
					stage = tryTail
				}
			}

			if !matched && stage == tryTail {
				if cur.tail != nil {
					// the trailing capture keeps its leading '/'
					pushVar(params, cur.tail.name, path[start-1:], maxVars)
					return cur.tail.chain, cur.tail.pattern, true
				}
			}
		}

		if matched {
			continue
		}

		// dead end — resume the most recent unfinished alternative
		if len(frames) == 0 {
			return nil, "", false
		}
		f := frames[len(frames)-1]
		frames = frames[:len(frames)-1]
		cur, start, stage, idx = f.n, f.start, f.stage, f.idx
		if params != nil {
			*params = (*params)[:f.nvars]
		}
	}
}

func varCount(params *Params) int {
	if params == nil {
		return 0
	}
	return len(*params)
}

// pushVar appends one captured parameter. The backing array is sized for
// the router's largest pattern on first growth, so a reused Params (and
// therefore the request hot path) allocates at most once.
func pushVar(params *Params, key, value string, maxVars int) {
	if params == nil {
		return
	}
	if cap(*params) <= len(*params) {
		want := maxVars
		if want < len(*params)+1 {
			want = len(*params) + 1
		}
		grown := make(Params, len(*params), want)
		copy(grown, *params)
		*params = grown
	}
	*params = append(*params, Param{Key: key, Value: value})
}
