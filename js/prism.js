/* PrismJS 1.29.0 - Lightweight syntax highlighting
 * https://prismjs.com/
 * Simplified version with Go, JavaScript, and Bash support
 */
(function() {
    var Prism = {
        languages: {},
        highlightAll: function() {
            var elements = document.querySelectorAll('code[class*="language-"]');
            elements.forEach(function(element) {
                Prism.highlightElement(element);
            });
        },
        highlightElement: function(element) {
            var code = element.textContent;
            var language = (element.className.match(/language-(\w+)/) || [, 'none'])[1];
            var grammar = Prism.languages[language];
            
            if (grammar) {
                element.innerHTML = Prism.highlight(code, grammar);
            }
        },
        highlight: function(code, grammar) {
            var tokens = Prism.tokenize(code, grammar);
            return Prism.stringify(tokens);
        },
        tokenize: function(code, grammar) {
            var tokens = [code];
            
            for (var token in grammar) {
                if (!grammar.hasOwnProperty(token)) continue;
                
                var pattern = grammar[token];
                var regex = pattern.pattern || pattern;
                
                for (var i = 0; i < tokens.length; i++) {
                    var str = tokens[i];
                    if (typeof str !== 'string') continue;
                    
                    var match;
                    var newTokens = [];
                    var lastIndex = 0;
                    
                    regex.lastIndex = 0;
                    while ((match = regex.exec(str)) !== null) {
                        if (match.index > lastIndex) {
                            newTokens.push(str.slice(lastIndex, match.index));
                        }
                        newTokens.push({
                            type: token,
                            content: match[0]
                        });
                        lastIndex = match.index + match[0].length;
                        
                        // Prevent infinite loops for zero-length matches
                        if (match[0].length === 0) {
                            regex.lastIndex++;
                        }
                    }
                    
                    if (lastIndex < str.length) {
                        newTokens.push(str.slice(lastIndex));
                    }
                    
                    if (newTokens.length > 0) {
                        tokens.splice(i, 1, ...newTokens);
                        i += newTokens.length - 1;
                    }
                }
            }
            
            return tokens;
        },
        stringify: function(tokens) {
            return tokens.map(function(token) {
                if (typeof token === 'string') {
                    return Prism.encode(token);
                }
                return '<span class="token ' + token.type + '">' + Prism.encode(token.content) + '</span>';
            }).join('');
        },
        encode: function(str) {
            return str
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
        }
    };

    // Go language
    Prism.languages.go = {
        'comment': { pattern: /\/\/.*|\/\*[\s\S]*?\*\//g },
        'string': { pattern: /"(?:[^"\\]|\\.)*"|`[^`]*`/g },
        'keyword': { pattern: /\b(?:break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)\b/g },
        'boolean': { pattern: /\b(?:true|false|nil|iota)\b/g },
        'function': { pattern: /\b[a-zA-Z_]\w*(?=\s*\()/g },
        'number': { pattern: /\b(?:0[xX][0-9a-fA-F]+|0[oO]?[0-7]+|0[bB][01]+|\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b/g },
        'operator': { pattern: /[+\-*/%&|^!<>=:]=?|&&|\|\||<-|<<|>>/g },
        'punctuation': { pattern: /[{}[\]();,.]/g }
    };

    // JavaScript language
    Prism.languages.javascript = Prism.languages.js = {
        'comment': { pattern: /\/\/.*|\/\*[\s\S]*?\*\//g },
        'string': { pattern: /"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|`(?:[^`\\]|\\.)*`/g },
        'keyword': { pattern: /\b(?:async|await|break|case|catch|class|const|continue|debugger|default|delete|do|else|export|extends|finally|for|from|function|if|import|in|instanceof|let|new|of|return|static|super|switch|this|throw|try|typeof|var|void|while|with|yield)\b/g },
        'boolean': { pattern: /\b(?:true|false|null|undefined|NaN|Infinity)\b/g },
        'function': { pattern: /\b[a-zA-Z_$]\w*(?=\s*\()/g },
        'number': { pattern: /\b(?:0[xX][0-9a-fA-F]+|0[oO][0-7]+|0[bB][01]+|\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b/g },
        'operator': { pattern: /[+\-*/%&|^!<>=?:]=?|&&|\|\||\.{3}|=>|\+\+|--/g },
        'punctuation': { pattern: /[{}[\]();,.]/g }
    };

    // Bash language
    Prism.languages.bash = Prism.languages.shell = {
        'comment': { pattern: /#.*/g },
        'string': { pattern: /"(?:[^"\\]|\\.)*"|'[^']*'/g },
        'keyword': { pattern: /\b(?:if|then|else|elif|fi|for|while|do|done|case|esac|function|in|select|until|return|exit)\b/g },
        'function': { pattern: /\b(?:alias|bg|bind|break|builtin|caller|cd|command|compgen|complete|compopt|continue|declare|dirs|disown|echo|enable|eval|exec|export|false|fc|fg|getopts|hash|help|history|jobs|kill|let|local|logout|mapfile|popd|printf|pushd|pwd|read|readarray|readonly|set|shift|shopt|source|suspend|test|times|trap|true|type|typeset|ulimit|umask|unalias|unset|wait)\b/g },
        'variable': { pattern: /\$(?:\w+|\{[^}]+\})/g },
        'operator': { pattern: /[|&;()<>]/g },
        'punctuation': { pattern: /[{}[\]]/g }
    };

    // Auto-highlight on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', Prism.highlightAll);
    } else {
        Prism.highlightAll();
    }

    window.Prism = Prism;
})();
