/* PrismJS 1.29.0 - Minimal Go syntax highlighting */
var Prism = (function() {
    var lang = /(?:^|\s)lang(?:uage)?-([\w-]+)(?=\s|$)/i;
    var uniqueId = 0;

    var _ = {
        manual: false,
        disableWorkerMessageHandler: false,
        util: {
            encode: function encode(tokens) {
                if (tokens instanceof Token) {
                    return new Token(tokens.type, encode(tokens.content), tokens.alias);
                } else if (Array.isArray(tokens)) {
                    return tokens.map(encode);
                } else {
                    return tokens.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/\u00a0/g, ' ');
                }
            },
            type: function(o) {
                return Object.prototype.toString.call(o).slice(8, -1);
            },
            objId: function(obj) {
                if (!obj['__id']) {
                    Object.defineProperty(obj, '__id', { value: ++uniqueId });
                }
                return obj['__id'];
            },
            clone: function deepClone(o, visited) {
                visited = visited || {};
                var clone, id;
                switch (_.util.type(o)) {
                    case 'Object':
                        id = _.util.objId(o);
                        if (visited[id]) return visited[id];
                        clone = {};
                        visited[id] = clone;
                        for (var key in o) {
                            if (o.hasOwnProperty(key)) {
                                clone[key] = deepClone(o[key], visited);
                            }
                        }
                        return clone;
                    case 'Array':
                        id = _.util.objId(o);
                        if (visited[id]) return visited[id];
                        clone = [];
                        visited[id] = clone;
                        o.forEach(function(v, i) {
                            clone[i] = deepClone(v, visited);
                        });
                        return clone;
                    default:
                        return o;
                }
            }
        },
        languages: {
            plain: {},
            plaintext: {},
            text: {},
            txt: {},
            extend: function(id, redef) {
                var lang = _.util.clone(_.languages[id]);
                for (var key in redef) {
                    lang[key] = redef[key];
                }
                return lang;
            },
            insertBefore: function(inside, before, insert, root) {
                root = root || _.languages;
                var grammar = root[inside];
                var ret = {};
                for (var token in grammar) {
                    if (grammar.hasOwnProperty(token)) {
                        if (token == before) {
                            for (var newToken in insert) {
                                if (insert.hasOwnProperty(newToken)) {
                                    ret[newToken] = insert[newToken];
                                }
                            }
                        }
                        if (!insert.hasOwnProperty(token)) {
                            ret[token] = grammar[token];
                        }
                    }
                }
                var old = root[inside];
                root[inside] = ret;
                _.languages.DFS(_.languages, function(key, value) {
                    if (value === old && key != inside) {
                        this[key] = ret;
                    }
                });
                return ret;
            },
            DFS: function DFS(o, callback, type, visited) {
                visited = visited || {};
                var objId = _.util.objId;
                for (var i in o) {
                    if (o.hasOwnProperty(i)) {
                        callback.call(o, i, o[i], type || i);
                        var property = o[i];
                        var propertyType = _.util.type(property);
                        if (propertyType === 'Object' && !visited[objId(property)]) {
                            visited[objId(property)] = true;
                            DFS(property, callback, null, visited);
                        } else if (propertyType === 'Array' && !visited[objId(property)]) {
                            visited[objId(property)] = true;
                            DFS(property, callback, i, visited);
                        }
                    }
                }
            }
        },
        plugins: {},
        highlightAll: function(async, callback) {
            _.highlightAllUnder(document, async, callback);
        },
        highlightAllUnder: function(container, async, callback) {
            var elements = container.querySelectorAll('code[class*="language-"], [class*="language-"] code');
            for (var i = 0, element; (element = elements[i++]);) {
                _.highlightElement(element, async === true, callback);
            }
        },
        highlightElement: function(element, async, callback) {
            var language = _.util.getLanguage(element);
            var grammar = _.languages[language];
            element.className = element.className.replace(lang, '').replace(/\s+/g, ' ') + ' language-' + language;
            var parent = element.parentElement;
            if (parent && parent.nodeName.toLowerCase() === 'pre') {
                parent.className = parent.className.replace(lang, '').replace(/\s+/g, ' ') + ' language-' + language;
            }
            var code = element.textContent;
            if (!grammar) {
                return;
            }
            element.innerHTML = _.highlight(code, grammar, language);
            if (callback) callback.call(element);
        },
        highlight: function(text, grammar, language) {
            var tokens = _.tokenize(text, grammar);
            return Token.stringify(_.util.encode(tokens), language);
        },
        tokenize: function(text, grammar) {
            var rest = grammar.rest;
            if (rest) {
                for (var token in rest) {
                    grammar[token] = rest[token];
                }
                delete grammar.rest;
            }
            var tokenList = new LinkedList();
            addAfter(tokenList, tokenList.head, text);
            matchGrammar(text, tokenList, grammar, tokenList.head, 0);
            return toArray(tokenList);
        }
    };

    _.util.getLanguage = function(element) {
        while (element) {
            var m = lang.exec(element.className);
            if (m) return m[1].toLowerCase();
            element = element.parentElement;
        }
        return 'none';
    };

    function Token(type, content, alias, matchedStr) {
        this.type = type;
        this.content = content;
        this.alias = alias;
        this.length = (matchedStr || '').length | 0;
    }

    Token.stringify = function stringify(o, language) {
        if (typeof o == 'string') return o;
        if (Array.isArray(o)) {
            var s = '';
            o.forEach(function(e) { s += stringify(e, language); });
            return s;
        }
        var env = {
            type: o.type,
            content: stringify(o.content, language),
            tag: 'span',
            classes: ['token', o.type],
            attributes: {},
            language: language
        };
        var aliases = o.alias;
        if (aliases) {
            if (Array.isArray(aliases)) {
                Array.prototype.push.apply(env.classes, aliases);
            } else {
                env.classes.push(aliases);
            }
        }
        var attributes = '';
        for (var name in env.attributes) {
            attributes += ' ' + name + '="' + (env.attributes[name] || '').replace(/"/g, '&quot;') + '"';
        }
        return '<' + env.tag + ' class="' + env.classes.join(' ') + '"' + attributes + '>' + env.content + '</' + env.tag + '>';
    };

    function LinkedList() {
        var head = { value: null, prev: null, next: null };
        var tail = { value: null, prev: head, next: null };
        head.next = tail;
        this.head = head;
        this.tail = tail;
        this.length = 0;
    }

    function addAfter(list, node, value) {
        var next = node.next;
        var newNode = { value: value, prev: node, next: next };
        node.next = newNode;
        next.prev = newNode;
        list.length++;
        return newNode;
    }

    function removeRange(list, node, count) {
        var next = node.next;
        for (var i = 0; i < count && next !== list.tail; i++) {
            next = next.next;
        }
        node.next = next;
        next.prev = node;
        list.length -= i;
    }

    function toArray(list) {
        var array = [];
        var node = list.head.next;
        while (node !== list.tail) {
            array.push(node.value);
            node = node.next;
        }
        return array;
    }

    function matchGrammar(text, tokenList, grammar, startNode, startPos, rematch) {
        for (var token in grammar) {
            if (!grammar.hasOwnProperty(token) || !grammar[token]) continue;
            var patterns = grammar[token];
            patterns = Array.isArray(patterns) ? patterns : [patterns];
            for (var j = 0; j < patterns.length; ++j) {
                if (rematch && rematch.cause == token + ',' + j) return;
                var patternObj = patterns[j];
                var inside = patternObj.inside;
                var lookbehind = !!patternObj.lookbehind;
                var greedy = !!patternObj.greedy;
                var alias = patternObj.alias;
                var pattern = patternObj.pattern || patternObj;
                for (var currentNode = startNode.next, pos = startPos; currentNode !== tokenList.tail; pos += currentNode.value.length, currentNode = currentNode.next) {
                    if (rematch && pos >= rematch.reach) break;
                    var str = currentNode.value;
                    if (tokenList.length > text.length) return;
                    if (str instanceof Token) continue;
                    var removeCount = 1;
                    var match;
                    if (greedy) {
                        match = matchPattern(pattern, pos, text, lookbehind);
                        if (!match || match.index >= text.length) break;
                        var from = match.index;
                        var to = match.index + match[0].length;
                        var p = pos;
                        p += currentNode.value.length;
                        while (from >= p) {
                            currentNode = currentNode.next;
                            p += currentNode.value.length;
                        }
                        p -= currentNode.value.length;
                        pos = p;
                        if (currentNode.value instanceof Token) continue;
                        for (var k = currentNode; k !== tokenList.tail && (p < to || typeof k.value === 'string'); k = k.next) {
                            removeCount++;
                            p += k.value.length;
                        }
                        removeCount--;
                        str = text.slice(pos, p);
                        match.index -= pos;
                    } else {
                        match = matchPattern(pattern, 0, str, lookbehind);
                        if (!match) continue;
                    }
                    var from = match.index;
                    var matchStr = match[0];
                    var before = str.slice(0, from);
                    var after = str.slice(from + matchStr.length);
                    var reach = pos + str.length;
                    if (rematch && reach > rematch.reach) rematch.reach = reach;
                    var removeFrom = currentNode.prev;
                    if (before) {
                        removeFrom = addAfter(tokenList, removeFrom, before);
                        pos += before.length;
                    }
                    removeRange(tokenList, removeFrom, removeCount);
                    var wrapped = new Token(token, inside ? _.tokenize(matchStr, inside) : matchStr, alias, matchStr);
                    currentNode = addAfter(tokenList, removeFrom, wrapped);
                    if (after) addAfter(tokenList, currentNode, after);
                    if (removeCount > 1) {
                        var nestedRematch = { cause: token + ',' + j, reach: reach };
                        matchGrammar(text, tokenList, grammar, currentNode.prev, pos, nestedRematch);
                        if (rematch && nestedRematch.reach > rematch.reach) rematch.reach = nestedRematch.reach;
                    }
                }
            }
        }
    }

    function matchPattern(pattern, pos, text, lookbehind) {
        pattern.lastIndex = pos;
        var match = pattern.exec(text);
        if (match && lookbehind && match[1]) {
            var lookbehindLength = match[1].length;
            match.index += lookbehindLength;
            match[0] = match[0].slice(lookbehindLength);
        }
        return match;
    }

    // Go language definition
    _.languages.go = {
        'comment': [
            { pattern: /\/\*[\s\S]*?(?:\*\/|$)/, greedy: true },
            { pattern: /\/\/.*/, greedy: true }
        ],
        'string': {
            pattern: /(^|[^\\])"(?:\\.|[^"\\\r\n])*"|`[^`]*`/,
            lookbehind: true,
            greedy: true
        },
        'keyword': /\b(?:break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)\b/,
        'boolean': /\b(?:_|false|iota|nil|true)\b/,
        'number': [
            /\b0(?:b[01_]+|o[0-7_]+)i?\b/i,
            /\b0x(?:[a-f\d_]+(?:\.[a-f\d_]*)?|\.[a-f\d_]+)(?:p[+-]?\d+(?:_\d+)*)?i?(?!\w)/i,
            /(?:\b\d[\d_]*(?:\.[\d_]*)?|\B\.\d[\d_]*)(?:e[+-]?[\d_]+)?i?(?!\w)/i
        ],
        'operator': /[*\/%^!=]=?|\+[=+]?|-[=-]?|\|[=|]?|&(?:=|&|\^=?)?|>(?:>=?|=)?|<(?:<=?|=|-)?|:=|\.\.\./,
        'builtin': /\b(?:append|bool|byte|cap|close|complex|complex128|complex64|copy|delete|error|float32|float64|imag|int|int16|int32|int64|int8|len|make|new|panic|print|println|real|recover|rune|string|uint|uint16|uint32|uint64|uint8|uintptr)\b/,
        'function': /\b\w+(?=\s*\()/,
        'punctuation': /[{}[\];(),.:]/
    };

    // Auto-highlight on DOM ready
    if (typeof document !== 'undefined') {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', _.highlightAll);
        } else {
            _.highlightAll();
        }
    }

    return _;
})();

if (typeof module !== 'undefined' && module.exports) {
    module.exports = Prism;
}
