package rue

import (
	"sync"
)

// Router is the high-performance router based on Radix Tree
type Router struct {
	trees      map[string]*node
	paramsPool sync.Pool
	maxParams  uint16
}

// nodeType represents the type of a node
type nodeType uint8

const (
	staticNode   nodeType = iota // static node
	rootNode                     // root node
	paramNode                    // parameter node :id
	catchAllNode                 // wildcard node *filepath
)

// node represents a node in the Radix Tree
type node struct {
	path      string
	indices   string
	wildChild bool
	nType     nodeType
	priority  uint32
	children  []*node
	handlers  HandlersChain
	fullPath  string
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
	return &Router{
		trees: make(map[string]*node),
	}
}

// addRoute registers a new route
func (r *Router) addRoute(method, path string, handlers HandlersChain) {
	if path[0] != '/' {
		panic("path must begin with '/'")
	}
	if method == "" {
		panic("HTTP method can not be empty")
	}
	if len(handlers) == 0 {
		panic("there must be at least one handler")
	}

	root := r.trees[method]
	if root == nil {
		root = &node{nType: rootNode}
		r.trees[method] = root
	}

	root.addRoute(path, handlers)
}

// getValue returns the handlers and params for a given path
func (r *Router) getValue(method, path string, params *Params) (HandlersChain, string, bool) {
	root := r.trees[method]
	if root == nil {
		return nil, "", false
	}
	return root.getValue(path, params)
}

// addRoute adds a route to the node
func (n *node) addRoute(path string, handlers HandlersChain) {
	fullPath := path
	n.priority++

	// Empty tree
	if len(n.path) == 0 && len(n.children) == 0 {
		n.insertChild(path, fullPath, handlers)
		n.nType = rootNode
		return
	}

	parentFullPathIndex := 0

walk:
	for {
		// Find the longest common prefix
		i := longestCommonPrefix(path, n.path)

		// Split edge
		if i < len(n.path) {
			child := node{
				path:      n.path[i:],
				wildChild: n.wildChild,
				nType:     staticNode,
				indices:   n.indices,
				children:  n.children,
				handlers:  n.handlers,
				priority:  n.priority - 1,
				fullPath:  n.fullPath,
			}

			n.children = []*node{&child}
			n.indices = string([]byte{n.path[i]})
			n.path = path[:i]
			n.handlers = nil
			n.wildChild = false
			n.fullPath = fullPath[:parentFullPathIndex+i]
		}

		// Make new node a child of this node
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// Check if a child with the next path byte exists
			for i, max := 0, len(n.indices); i < max; i++ {
				if c == n.indices[i] {
					parentFullPathIndex += len(n.path)
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// Otherwise insert it
			if c != ':' && c != '*' {
				n.indices += string([]byte{c})
				child := &node{
					fullPath: fullPath,
				}
				n.children = append(n.children, child)
				n.incrementChildPrio(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				// Check if the wildcard matches
				n = n.children[len(n.children)-1]
				n.priority++

				// Check if the wildcard matches
				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					n.nType != catchAllNode &&
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}

				panic("conflict with wildcard route")
			}

			n.insertChild(path, fullPath, handlers)
			return
		}

		// Otherwise add handle to current node
		if n.handlers != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handlers = handlers
		n.fullPath = fullPath
		return
	}
}

// insertChild inserts a child node
func (n *node) insertChild(path, fullPath string, handlers HandlersChain) {
	for {
		// Find prefix until first wildcard
		wildcard, i, valid := findWildcard(path)
		if i < 0 {
			break
		}

		if !valid {
			panic("only one wildcard per path segment is allowed")
		}

		if len(wildcard) < 2 {
			panic("wildcards must be named with a non-empty name")
		}

		if wildcard[0] == ':' {
			// Param
			if i > 0 {
				n.path = path[:i]
				path = path[i:]
			}

			child := &node{
				nType:    paramNode,
				path:     wildcard,
				fullPath: fullPath,
			}
			n.wildChild = true
			n.children = append(n.children, child)
			n = child
			n.priority++

			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &node{
					priority: 1,
					fullPath: fullPath,
				}
				n.children = append(n.children, child)
				n = child
				continue
			}

			n.handlers = handlers
			return
		}

		// catchAll
		if i+len(wildcard) != len(path) {
			panic("catch-all routes are only allowed at the end of the path")
		}

		if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			panic("catch-all conflicts with existing handle for the path segment root")
		}

		i--
		if path[i] != '/' {
			panic("no / before catch-all in path '" + fullPath + "'")
		}

		n.path = path[:i]

		// First node: catchAll node with empty path
		child := &node{
			wildChild: true,
			nType:     catchAllNode,
			fullPath:  fullPath,
		}
		n.children = append(n.children, child)
		n.indices = string('/')
		n = child
		n.priority++

		// Second node: node holding the variable
		child = &node{
			path:     path[i:],
			nType:    catchAllNode,
			handlers: handlers,
			priority: 1,
			fullPath: fullPath,
		}
		n.children = append(n.children, child)

		return
	}

	n.path = path
	n.handlers = handlers
	n.fullPath = fullPath
}

// getValue returns the handlers for a path
func (n *node) getValue(path string, params *Params) (handlers HandlersChain, fullPath string, found bool) {
walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				// Try all the non-wildcard children first
				idxc := path[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
						n = n.children[i]
						continue walk
					}
				}

				// If there is no wildcard pattern, we're done
				if !n.wildChild {
					return nil, "", false
				}

				// Handle wildcard child
				n = n.children[len(n.children)-1]

				switch n.nType {
				case paramNode:
					// Find param end
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}

					// Save param value
					if params != nil {
						if cap(*params) < int(n.priority) {
							*params = make(Params, 0, n.priority)
						}
						*params = append(*params, Param{
							Key:   n.path[1:],
							Value: path[:end],
						})
					}

					// Continue deeper
					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}
						return nil, "", false
					}

					if handlers = n.handlers; handlers != nil {
						fullPath = n.fullPath
						found = true
						return
					}
					return nil, "", false

				case catchAllNode:
					// Save param value
					if params != nil {
						if cap(*params) < 1 {
							*params = make(Params, 0, 1)
						}
						*params = append(*params, Param{
							Key:   n.path[2:],
							Value: path,
						})
					}

					handlers = n.handlers
					fullPath = n.fullPath
					found = true
					return

				default:
					panic("invalid node type")
				}
			}
		}

		if path == prefix {
			if handlers = n.handlers; handlers != nil {
				fullPath = n.fullPath
				found = true
				return
			}
		}

		return nil, "", false
	}
}

// incrementChildPrio increments priority of the given child and reorders if necessary
func (n *node) incrementChildPrio(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	// Adjust position (move to front)
	newPos := pos
	for ; newPos > 0 && cs[newPos-1].priority < prio; newPos-- {
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
	}

	// Build new index char string
	if newPos != pos {
		n.indices = n.indices[:newPos] +
			n.indices[pos:pos+1] +
			n.indices[newPos:pos] +
			n.indices[pos+1:]
	}

	return newPos
}

// longestCommonPrefix finds the longest common prefix
func longestCommonPrefix(a, b string) int {
	i := 0
	max := min(len(a), len(b))
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

// findWildcard finds a wildcard segment
func findWildcard(path string) (wildcard string, i int, valid bool) {
	for start, c := range []byte(path) {
		if c != ':' && c != '*' {
			continue
		}

		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}
