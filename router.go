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
	roots   methodRoots                     // method → segment-trie root
	statics map[string]map[string]staticHit // optional exact-match fast path; nil = disabled
	maxVars int                             // largest capture count of any registered pattern
}

// methodRoots stores the per-method trie roots. The common methods live in
// plain fields — a string switch beats a map hash on the lookup hot path —
// and anything else (CONNECT, TRACE, custom verbs) falls back to a map.
type methodRoots struct {
	get, post, put, delete, patch, head, options *segNode
	other                                        map[string]*segNode
}

// root returns the trie root registered for method, or nil.
func (m *methodRoots) root(method string) *segNode {
	switch method {
	case "GET":
		return m.get
	case "POST":
		return m.post
	case "PUT":
		return m.put
	case "DELETE":
		return m.delete
	case "PATCH":
		return m.patch
	case "HEAD":
		return m.head
	case "OPTIONS":
		return m.options
	}
	return m.other[method]
}

// ensure returns the trie root for method, creating it on first use.
func (m *methodRoots) ensure(method string) *segNode {
	if root := m.root(method); root != nil {
		return root
	}
	root := &segNode{}
	switch method {
	case "GET":
		m.get = root
	case "POST":
		m.post = root
	case "PUT":
		m.put = root
	case "DELETE":
		m.delete = root
	case "PATCH":
		m.patch = root
	case "HEAD":
		m.head = root
	case "OPTIONS":
		m.options = root
	default:
		if m.other == nil {
			m.other = make(map[string]*segNode)
		}
		m.other[method] = root
	}
	return root
}

// staticHit is one entry of the optional exact-match fast path.
type staticHit struct {
	chain   HandlersChain
	pattern string
}

// segNode is one position in the trie. A node owns up to four kinds of
// children, tried most-specific-first during a lookup: exact literals,
// prefixed captures (longest prefix first), one bare capture, and one
// trailing capture. A trailing capture cannot share a position with any
// other child kind — the position would be ambiguous.
//
// Literal children are run-compressed: a label spans as many consecutive
// literal segments as the route shape allows ("api/v1/users"), so a
// purely static route usually resolves with a single string comparison.
// Labels never start with '/'; runs split only at segment boundaries.
type segNode struct {
	label      string     // literal run (statics) or literal prefix (mixes)
	name       string     // capture name (mixes, capture, tail)
	statics    []*segNode // literal children; first segments are unique
	firstChars []byte     // firstChars[i] is statics[i].label's first byte (0 if empty)
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
	return &Router{}
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

	root := r.roots.ensure(method)

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
	root := r.roots.root(method)
	if root == nil || len(path) == 0 || path[0] != '/' {
		return nil, "", false
	}

	// Fast walk: literal-only levels need no backtracking state, so pure
	// static lookups stay free of bookkeeping. The walk hands off to the
	// dynamic resolver at the first node owning any capture child.
	cur := root
	start := 1
	for cur.mixes == nil && cur.capture == nil && cur.tail == nil {
		if start > len(path) {
			if cur.chain != nil {
				return cur.chain, cur.pattern, true
			}
			return nil, "", false
		}
		var c byte
		if start < len(path) && path[start] != '/' {
			c = path[start]
		}
		var next *segNode
		for i, fc := range cur.firstChars {
			if fc != c {
				continue
			}
			l := cur.statics[i].label
			stop := start + len(l)
			if stop <= len(path) && path[start:stop] == l && (stop == len(path) || path[stop] == '/') {
				next = cur.statics[i]
				start = stop + 1
				break
			}
		}
		if next == nil {
			return nil, "", false
		}
		cur = next
	}
	return cur.resolveDynamic(path, start, params, r.maxVars)
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
			// extend the literal run across every capture-free segment
			stop := end
			for stop < len(pattern) {
				e := stop + 1
				literal := true
				for ; e < len(pattern) && pattern[e] != '/'; e++ {
					if pattern[e] == ':' || pattern[e] == '*' {
						literal = false
					}
				}
				if !literal {
					break
				}
				stop = e
			}
			cur = cur.mountRun(pattern[start:stop], pattern)
			start = stop + 1
			continue

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

// mountRun inserts a literal run (one or more '/'-joined capture-free
// segments) below n, descending through, splitting, or creating
// run-compressed literal children, and returns the node at the run's end.
func (n *segNode) mountRun(run, pattern string) *segNode {
	cur := n
	for {
		child := cur.staticBySegment(run)
		if child == nil {
			if cur.tail != nil {
				panic("rue: pattern '" + pattern + "' conflicts with capture '*" + cur.tail.name + "' registered at this position")
			}
			child = &segNode{label: run}
			cur.addStatic(child)
			return child
		}

		// advance a shared cursor across label and run; segments stay in
		// lockstep because only equal segments let the cursor move
		l := child.label
		p := 0
		for {
			le, re := p, p
			for le < len(l) && l[le] != '/' {
				le++
			}
			for re < len(run) && run[re] != '/' {
				re++
			}
			if le != re || l[p:le] != run[p:re] {
				// diverging segments: fork the label at the last shared boundary
				junction := splitStatic(child, p-1)
				if p > len(run) {
					return junction // unreachable; kept for symmetry
				}
				rest := &segNode{label: run[p:]}
				junction.addStatic(rest)
				return rest
			}
			switch {
			case le == len(l) && re == len(run):
				return child // label and run end together
			case le == len(l):
				// label exhausted, run continues below this child
				cur = child
				run = run[re+1:]
			case re == len(run):
				// run ends inside the label: fork and stop at the junction
				return splitStatic(child, re)
			default:
				p = le + 1
				continue
			}
			break
		}
	}
}

// staticBySegment returns the literal child whose first segment equals the
// first segment of run, if any. First segments are unique among siblings.
func (n *segNode) staticBySegment(run string) *segNode {
	e := 0
	for e < len(run) && run[e] != '/' {
		e++
	}
	var c byte
	if e > 0 {
		c = run[0]
	}
	for i, fc := range n.firstChars {
		if fc != c {
			continue
		}
		l := n.statics[i].label
		if len(l) >= e && l[:e] == run[:e] && (len(l) == e || l[e] == '/') {
			return n.statics[i]
		}
	}
	return nil
}

// splitStatic forks child at byte offset at (which must sit on a segment
// boundary): the child keeps label[:at] and becomes a plain junction, and a
// new node holding label[at+1:] inherits everything else.
func splitStatic(child *segNode, at int) *segNode {
	deep := &segNode{
		label:      child.label[at+1:],
		statics:    child.statics,
		firstChars: child.firstChars,
		mixes:      child.mixes,
		capture:    child.capture,
		tail:       child.tail,
		chain:      child.chain,
		pattern:    child.pattern,
	}
	child.label = child.label[:at]
	child.statics = nil
	child.firstChars = nil
	child.mixes = nil
	child.capture = nil
	child.tail = nil
	child.chain = nil
	child.pattern = ""
	child.addStatic(deep)
	return child
}

func (n *segNode) addStatic(child *segNode) {
	var c byte
	if len(child.label) > 0 && child.label[0] != '/' {
		c = child.label[0]
	}
	n.statics = append(n.statics, child)
	n.firstChars = append(n.firstChars, c)
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

// matchFrame is a resume point for backtracking: retry node n at path
// offset start, continuing with alternative kind stage at child index idx,
// after truncating captured params back to nvars.
type matchFrame struct {
	n     *segNode
	start int32
	nvars int32
	stage uint8
	idx   uint16
}

// Alternative kinds, in lookup priority order.
const (
	tryStatics uint8 = iota
	tryMixes
	tryCapture
	tryTail
)

// resolveDynamic walks path below n starting at byte offset start. At every
// node the alternatives are tried most-specific-first; when a branch
// dead-ends the walk resumes at the most recent node that still has untried
// alternatives, so a miss under a literal child falls through to a capture
// sibling. A resume point is recorded only when such alternatives exist.
func (n *segNode) resolveDynamic(path string, start int, params *Params, maxVars int) (HandlersChain, string, bool) {
	var room [4]matchFrame // usually enough; spills to the heap when exceeded
	frames := room[:0]

	cur := n
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
			if stage == tryStatics {
				var c byte
				if start < len(path) && path[start] != '/' {
					c = path[start]
				}
				for i, fc := range cur.firstChars {
					if fc != c {
						continue
					}
					l := cur.statics[i].label
					stop := start + len(l)
					if stop <= len(path) && path[start:stop] == l && (stop == len(path) || path[stop] == '/') {
						if cur.mixes != nil || cur.capture != nil || cur.tail != nil {
							frames = append(frames, matchFrame{cur, int32(start), int32(varCount(params)), tryMixes, 0})
						}
						cur = cur.statics[i]
						start = stop + 1
						matched = true
						break
					}
				}
				if !matched {
					stage, idx = tryMixes, 0
				}
			}

			if !matched && stage != tryStatics {
				end := start
				for end < len(path) && path[end] != '/' {
					end++
				}
				seg := path[start:end]

				if stage == tryMixes {
					for ; idx < len(cur.mixes); idx++ {
						m := cur.mixes[idx]
						if len(seg) > len(m.label) && seg[:len(m.label)] == m.label {
							nvars := varCount(params)
							if idx+1 < len(cur.mixes) || cur.capture != nil || cur.tail != nil {
								frames = append(frames, matchFrame{cur, int32(start), int32(nvars), tryMixes, uint16(idx + 1)})
							}
							pushVar(params, m.name, seg[len(m.label):], maxVars)
							cur = m
							start = end + 1
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
							frames = append(frames, matchFrame{cur, int32(start), int32(varCount(params)), tryTail, 0})
						}
						pushVar(params, cur.capture.name, seg, maxVars)
						cur = cur.capture
						start = end + 1
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
		}

		if matched {
			stage, idx = tryStatics, 0
			continue
		}

		// dead end — resume the most recent unfinished alternative
		if len(frames) == 0 {
			return nil, "", false
		}
		f := frames[len(frames)-1]
		frames = frames[:len(frames)-1]
		cur, start, stage, idx = f.n, int(f.start), f.stage, int(f.idx)
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
