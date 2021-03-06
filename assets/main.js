(function(g) {
    /*
     *  Copyright 2012-2013 (c) Pierre Duquesne <stackp@online.fr>
     *  Licensed under the New BSD License.
     *  https://github.com/stackp/promisejs
     */
    {
        function Promise() {
            this._callbacks = [];
        }

        Promise.prototype.then = function(func, context) {
            var p;
            if (this._isdone) {
                p = func.apply(context, this.result);
            } else {
                p = new Promise();
                this._callbacks.push(function () {
                    var res = func.apply(context, arguments);
                    if (res && typeof res.then === 'function')
                        res.then(p.done, p);
                });
            }
            return p;
        };

        Promise.prototype.done = function() {
            this.result = arguments;
            this._isdone = true;
            for (var i = 0; i < this._callbacks.length; i++) {
                this._callbacks[i].apply(null, arguments);
            }
            this._callbacks = [];
        };

        function join(promises) {
            var p = new Promise();
            var results = [];

            if (!promises || !promises.length) {
                p.done(results);
                return p;
            }

            var numdone = 0;
            var total = promises.length;

            function notifier(i) {
                return function() {
                    numdone += 1;
                    results[i] = Array.prototype.slice.call(arguments);
                    if (numdone === total) {
                        p.done(results);
                    }
                };
            }

            for (var i = 0; i < total; i++) {
                promises[i].then(notifier(i));
            }

            return p;
        }

        function chain(funcs, args) {
            var p = new Promise();
            if (funcs.length === 0) {
                p.done.apply(p, args);
            } else {
                funcs[0].apply(null, args).then(function() {
                    funcs.splice(0, 1);
                    chain(funcs, arguments).then(function() {
                        p.done.apply(p, arguments);
                    });
                });
            }
            return p;
        }

        /*
         * AJAX requests
         */

        function _encode(data) {
            var payload = "";
            if (typeof data === "string") {
                payload = data;
            } else {
                var e = encodeURIComponent;
                var params = [];

                for (var k in data) {
                    if (data.hasOwnProperty(k)) {
                        params.push(e(k) + '=' + e(data[k]));
                    }
                }
                payload = params.join('&')
            }
            return payload;
        }

        function new_xhr() {
            var xhr;
            if (window.XMLHttpRequest) {
                xhr = new XMLHttpRequest();
            } else if (window.ActiveXObject) {
                try {
                    xhr = new ActiveXObject("Msxml2.XMLHTTP");
                } catch (e) {
                    xhr = new ActiveXObject("Microsoft.XMLHTTP");
                }
            }
            return xhr;
        }


        function ajax(method, url, data, headers) {
            var p = new Promise();
            var xhr, payload;
            data = data || {};
            headers = headers || {};

            try {
                xhr = new_xhr();
            } catch (e) {
                p.done(promise.ENOXHR, "");
                return p;
            }

            var content_type = 'application/x-www-form-urlencoded';

            if (method[0] == "$") {
                method = method.substring(1);
                payload = new FormData();
                content_type = '';

                for (var k in data) {
                    if (data.hasOwnProperty(k)) payload.append(k, data[k]);
                }
            } else {
                payload = _encode(data);
                if (method === 'GET' && payload) {
                    url += '?' + payload;
                    payload = null;
                }
            }

            xhr.open(method, url);
  
            for (var h in headers) {
                if (headers.hasOwnProperty(h)) {
                    if (h.toLowerCase() === 'content-type')
                        content_type = headers[h];
                    else
                        xhr.setRequestHeader(h, headers[h]);
                }
            }
            if (content_type != '') xhr.setRequestHeader('Content-type', content_type);


            function onTimeout() {
                xhr.abort();
                p.done(promise.ETIMEOUT, "", xhr);
            }

            var timeout = promise.ajaxTimeout;
            if (timeout) {
                var tid = setTimeout(onTimeout, timeout);
            }

            xhr.onreadystatechange = function() {
                if (timeout) {
                    clearTimeout(tid);
                }
                if (xhr.readyState === 4) {
                    var err = (!xhr.status ||
                               (xhr.status < 200 || xhr.status >= 300) &&
                               xhr.status !== 304);
                    p.done(err, xhr.responseText, xhr);
                }
            };

            xhr.send(payload);
            return p;
        }

        function _ajaxer(method) {
            return function(url, data, headers) {
                return ajax(method, url, data, headers);
            };
        }

        var promise = {
            Promise: Promise,
            join: join,
            chain: chain,
            ajax: ajax,
            get: _ajaxer('GET'),
            post: _ajaxer('POST'),
            $post: _ajaxer('$POST'),
            put: _ajaxer('PUT'),
            del: _ajaxer('DELETE'),

            /* Error codes */
            ENOXHR: 1,
            ETIMEOUT: 2,

            ajaxTimeout: 20000
        };
    }

    var _id = function(id) {
        var e = (typeof id === 'string' || id instanceof String) ? document.getElementById(id) : id;
        if (e == null) return {};

        e.on = function(n, f, c) {
            if (e.addEventListener) {
                e.addEventListener(n, f, c);
            } else {
                e.attachEvent("on" + n, f);
            }

            return e;
        }
        e.html = function(html) {
            if (typeof html === 'string')
                e.innerHTML = html;
            else if (html.innerHTML) 
                e.innerHTML = html.innerHTML;

            return e;
        }
        e.attr = function(a, b) {
            if (b) {
                e.setAttribute(a, b);
                return e;
            } else {
                return e.getAttribute(a);
            }
        }

        return e;
    };

    var _editorId = null;

    var _insideEditor = function() {
        var __getSelected = function(isStart) {
            var range, sel, container;
            if (document.selection) {
                range = document.selection.createRange();
                range.collapse(isStart);
                return range.parentElement();
            } else {
                sel = window.getSelection();
                if (sel.getRangeAt) {
                    if (sel.rangeCount > 0) {
                        range = sel.getRangeAt(0);
                    }
                } else {
                    // Old WebKit
                    range = document.createRange();
                    range.setStart(sel.anchorNode, sel.anchorOffset);
                    range.setEnd(sel.focusNode, sel.focusOffset);

                    // Handle the case when the selection was selected backwards (from the end to the start in the document)
                    if (range.collapsed !== sel.isCollapsed) {
                        range.setStart(sel.focusNode, sel.focusOffset);
                        range.setEnd(sel.anchorNode, sel.anchorOffset);
                    }
               }

                if (range) {
                   container = range[isStart ? "startContainer" : "endContainer"];

                   // Check if the container is a text node and return its parent if so
                   return container.nodeType === 3 ? container.parentNode : container;
                }   
            }
        };

        var __iter = function (elem) {
            if(elem && elem.getAttribute){
                if (elem.getAttribute("contenteditable") == "true") return true;
                if (elem.parentNode) {
                    return __iter(elem.parentNode);
                }
            }
            return false;
        }

        return __iter(__getSelected());
    };

    var _insertHTML = function(html) {
        // http://stackoverflow.com/questions/6690752/insert-html-at-caret-in-a-contenteditable-div/6691294#6691294
        var sel, range;
        if (window.getSelection) {
            // IE9 and non-IE
            sel = window.getSelection();
            if (sel.getRangeAt && sel.rangeCount) {
                range = sel.getRangeAt(0);
                range.deleteContents();

                // Range.createContextualFragment() would be useful here but is
                // only relatively recently standardized and is not supported in
                // some browsers (IE9, for one)
                var el = document.createElement("div");
                el.innerHTML = html;
                var frag = document.createDocumentFragment(), node, lastNode;
                while ( (node = el.firstChild) ) {
                    lastNode = frag.appendChild(node);
                }
                range.insertNode(frag);

                // Preserve the selection
                if (lastNode) {
                    range = range.cloneRange();
                    range.setStartAfter(lastNode);
                    range.collapse(true);
                    sel.removeAllRanges();
                    sel.addRange(range);
                }
            }
        } else if (document.selection && document.selection.type != "Control") {
            // IE < 9
            document.selection.createRange().pasteHTML(html);
        }
    };

    g.etc = {
        "id": _id,

        "file": function(id, ev) {
            if (id[0] && id[0].type && id.length) {
                var ret = [];
                for (var i = 0; i < id.length; i++) ret.push(id[i]);
                return ret;
            }

            var f = _id(id);
            if (f.files && f.files[0]) {
                var ret = [];
                for (var i = 0; i < f.files.length; i++) ret.push(f.files[i]);

                return ret;
            }
            else if (ev && ev.target && ev.target.value)
                return [ev.target.value];
            else
                return [];
        },

        "get": function(selector) { return document.querySelectorAll(selector); },

        "let": {
            "hide": function(id) {
                g.etc.let.$ = _id(id);
                g.etc.let.$.style.display = "none";
                return g.etc.let;
            },

            "show": function(id) {
                g.etc.let.$ = _id(id);
                var e = g.etc.let.$;
                switch (e.tagName.toLowerCase()) {
                case "td":
                    e.style.display = "table-cell";
                    break;
                case "table":
                    e.style.display = "table";
                    break;
                case "a":
                case "span":
                    e.style.display = "inherit";
                    break;
                default:
                    e.style.display = "block";
                }
                return g.etc.let;
            },

            "disable": function(id) { 
                g.etc.let.$ = _id(id);
                g.etc.let.$.disabled = true; 
                return g.etc.let;
            },

            "enable": function(id) { 
                g.etc.let.$ = _id(id);
                g.etc.let.$.disabled = false; 
                return g.etc.let;
            }
        },

        "date": {
            "format": function(timestamp) {
                var d = new Date(timestamp);
                var today = new Date();
                var yyyy = d.getFullYear();
                var mm = d.getMonth() < 9 ? "0" + (d.getMonth() + 1) : (d.getMonth() + 1);
                var dd = d.getDate() < 10 ? "0" + d.getDate() : d.getDate();
                var hh = d.getHours() < 10 ? "0" + d.getHours() : d.getHours();
                var min = d.getMinutes() < 10 ? "0" + d.getMinutes() : d.getMinutes();
                var ss = d.getSeconds() < 10 ? "0" + d.getSeconds() : d.getSeconds();

                ret = yyyy + "/" + mm + "/" + dd + " " + hh + ":" + min;
                return ret;
            },

            "now": function() {
                var d = new Date();
                var hh = d.getHours() < 10 ? "0" + d.getHours() : d.getHours();
                var min = d.getMinutes() < 10 ? "0" + d.getMinutes() : d.getMinutes();
                var ss = d.getSeconds() < 10 ? "0" + d.getSeconds() : d.getSeconds();
                return hh + ":" + min + ":" + ss;
            }
        },

        "string": {
            "unescape": function(str) {
                var e = document.createElement('div');
                e.innerHTML = str;
                var result = "";
                for (var i = 0; i < e.childNodes.length; ++i) {
                    result += e.childNodes[i].nodeValue;
                }
                return result;
            },

            "utf8Len": function(str) {
                // Matches only the 10.. bytes that are non-initial characters in a multi-byte sequence.
                if (encodeURIComponent) {
                    var m = encodeURIComponent(str).match(/%[89ABab]/g);
                    return str.length + (m ? m.length : 0);
                } else 
                    return 0;
            },

            "base64Decode": function(b64) {
                /*
                 * Copyright (c) 2010 Nick Galbreath
                 * See full license on http://code.google.com/p/stringencoders/source/browse/#svn/trunk/javascript
                 */
                var base64 = {};
                base64.PADCHAR = '=';
                base64.ALPHA = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';

                base64.makeDOMException = function() {
                    // sadly in FF,Safari,Chrome you can't make a DOMException
                    var e, tmp;

                    try {
                        return new DOMException(DOMException.INVALID_CHARACTER_ERR);
                    } catch (tmp) {
                        // not available, just passback a duck-typed equiv
                        // https://developer.mozilla.org/en/Core_JavaScript_1.5_Reference/Global_Objects/Error
                        // https://developer.mozilla.org/en/Core_JavaScript_1.5_Reference/Global_Objects/Error/prototype
                        var ex = new Error("DOM Exception 5");

                        // ex.number and ex.description is IE-specific.
                        ex.code = ex.number = 5;
                        ex.name = ex.description = "INVALID_CHARACTER_ERR";

                        // Safari/Chrome output format
                        ex.toString = function() { return 'Error: ' + ex.name + ': ' + ex.message; };
                        return ex;
                    }
                }

                base64.getbyte64 = function(s,i) {
                    // This is oddly fast, except on Chrome/V8.
                    //  Minimal or no improvement in performance by using a
                    //   object with properties mapping chars to value (eg. 'A': 0)
                    var idx = base64.ALPHA.indexOf(s.charAt(i));
                    if (idx === -1) {
                        throw base64.makeDOMException();
                    }
                    return idx;
                }

                base64.decode = function(s) {
                    // convert to string
                    s = '' + s;
                    var getbyte64 = base64.getbyte64;
                    var pads, i, b10;
                    var imax = s.length
                    if (imax === 0) {
                        return s;
                    }

                    if (imax % 4 !== 0) {
                        throw base64.makeDOMException();
                    }

                    pads = 0
                    if (s.charAt(imax - 1) === base64.PADCHAR) {
                        pads = 1;
                        if (s.charAt(imax - 2) === base64.PADCHAR) {
                            pads = 2;
                        }
                        // either way, we want to ignore this last block
                        imax -= 4;
                    }

                    var x = [];
                    for (i = 0; i < imax; i += 4) {
                        b10 = (getbyte64(s,i) << 18) | (getbyte64(s,i+1) << 12) |
                            (getbyte64(s,i+2) << 6) | getbyte64(s,i+3);
                        x.push(String.fromCharCode(b10 >> 16, (b10 >> 8) & 0xff, b10 & 0xff));
                    }

                    switch (pads) {
                    case 1:
                        b10 = (getbyte64(s,i) << 18) | (getbyte64(s,i+1) << 12) | (getbyte64(s,i+2) << 6);
                        x.push(String.fromCharCode(b10 >> 16, (b10 >> 8) & 0xff));
                        break;
                    case 2:
                        b10 = (getbyte64(s,i) << 18) | (getbyte64(s,i+1) << 12);
                        x.push(String.fromCharCode(b10 >> 16));
                        break;
                    }
                    return x.join('');
                }

                return base64.decode(b64);
            },
        },

        "util": {
            "ajax": promise,

            "isNumber": function(id) {
                return !isNaN(parseFloat(id)) && isFinite(id) && parseInt(id) > 0;
            },

            "CSRF": function() { return _id('csrf').innerHTML; },

            "lazyLoad": function(source, callback) {
                var _prefix = window.__cdn;
                var _load = function() {
                    var _gotoNext = function() {
                        source.shift();
                        if (source.length > 0) 
                            _load();
                        else {
                            if (callback) callback();
                        }
                    }

                    var s = null;
                    var _src = _prefix + source[0];

                    if (/\.js$/.test(_src)) {
                        s = document.createElement('script');
                        var scripts = document.querySelectorAll('script');
                        // console.log(scripts);
                        for(var i in scripts) {
                            if (scripts[i].src == _src) _gotoNext();
                        }

                        s.src = _src;
                        s.async = true;
                        s.onreadystatechange = s.onload = function() {
                            if (!s.readyState || /loaded|complete/.test(s.readyState)) {
                                _gotoNext();
                            }
                        };
                    } else if (/\.css$/.test(_src)) {
                        s = document.createElement('link');
                        var links = document.querySelectorAll('link');
                        
                        for(var i in links) {
                            if (links[i].href == _src) _gotoNext();
                        }                    

                        s.rel  = 'stylesheet';
                        s.type = 'text/css';
                        s.href = _src;
                        s.media = 'all';

                        _gotoNext();
                    }

                    if (s != null)
                        document.querySelector('head').appendChild(s);
                }

                _load();
            },

            "read": function(key) {
                if (window.attachEvent && !window.addEventListener) {
                    // IE8, return
                    return null;
                }

                if (localStorage) {
                    return localStorage.getItem(key);
                }

                return null;
            },

            "write": function(key, value) {
                if (window.attachEvent && !window.addEventListener) {
                    // IE8, return
                    return null;
                }

                if (localStorage) {
                    return localStorage.setItem(key, value);
                }

                return null;
            },

            "storage": {
                "remove": function(key) {
                    if (window.attachEvent && !window.addEventListener) {
                        // IE8, return
                        return null;
                    }

                    if (localStorage) {
                        localStorage.removeItem(key);
                    }
                },
            },
        },
        
        "editor": {
            "wrap": function(id) { _editorId = id; },

            "insertHTML": _insertHTML,

            "isInside": _insideEditor,

            "uploadImage": function(files, callback, options) {
                options = options || {};

                if (files.length == 0) {
                    callback();
                    return;
                }

                var file = files.shift();

                var onError = function(A) {
                    alert("Err::Upload::" + A);
                    if (callback) callback();
                };

                if (!_insideEditor() && options["editor"]) {
                    if (callback) callback();
                    return;
                }

                if (!file || !file.type.match(/image.*/)) {
                    if (/\.(jpg|png)\-(small|large)/.test(file.name)) {
                        // Twitter images
                    } else {
                        onError("Invalid_Image");
                        return
                    }
                }

                var _URL = window.URL || window.webkitURL;
                var img = new Image();

                var onLoad = function(w, h) {
                    etc.util.ajax.$post(options["imgur"] ? "https://api.imgur.com/3/image" : "/upload",
                    {
                        "image": file,
                        "width": w,
                        "height": h,
                    }, {
                        'Authorization': 'Client-ID c37fc05199a05b7'
                    }).then(function(e, rt, x) {
                        if (e) {
                            onError("AJAX");
                            return;
                        }
                        try {
                            var D = options["imgur"] ? JSON.parse(rt).data : JSON.parse(rt);
                        } catch (E) {
                            return onError("JSON");
                        }

                        if (D.Error || D.error) {
                            return onError("Server_Failure_" + D.R || D.error);
                        }

                        var _link = (D.Link || D.link).replace("http://", "https://");
                        var _thumb = (D.Thumbnail || D.link).replace("http://", "https://");

                        if (options["editor"]) {
                            _id(options["editor"]).focus();
                            _insertHTML("<a href='" + _link +
                                "' target='_blank'><img src='" + _thumb + "' class='article-image'></a>");
                        }
                        etc.editor.uploadImage(files, callback, options);
                    });
                }

                img.onload = function() {
                    onLoad(this.width, this.height);
                };

                if (_URL && _URL.createObjectURL)
                    img.src = _URL.createObjectURL(file);
                else
                    onLoad(1024, 1024);
            },

            "insertLink": function() {
                if (!_insideEditor()) return;

                var A = prompt("URL:", "http://");//this.inputs.href.value;
                var B = (window.getSelection) ? window.getSelection() : document.selection;

                if (A == "http://" || A == "") return;
                if (B == "" || B == null) B = prompt("显示文字:", A);

                _id(_editorId).focus();
                _insertHTML("<a href='" + A + "' target='_blank'>" + B + "</a>");
            },

            "insertList": function(elem) {
                _id(_editorId).focus();

                if (elem == 'ol')
                    document.execCommand("insertorderedlist", false);
                else
                    document.execCommand("insertunorderedlist", false);
            },

            "insertNode": function(cmd, arg) {
                _id(_editorId).focus();
                if (cmd == "heading")
                    document.execCommand("formatBlock", false, "<" + arg + ">");
                else
                    document.execCommand(cmd, false, arg);
            },

            "insertTable": function() {
                var _size = prompt("Cols x Rows:", "3x2");
                var size = _size.split("x");
                if (size.length < 2) return;

                var cols = parseInt(size[0]), rows = parseInt(size[1]);
                var row = "<tr>" + (new Array(cols + 1).join("<td>@</td>")) + "</tr>";
                var table = "<table border=1>" + new Array(rows + 1).join(row) + "</table>";

                _insertHTML(table);
            },
        },
    };
})(this);

var Gallery = (function() {
    return {
        "_Normal": function() {
        	etc.id("gallery").style.display = "none";
            etc.id("article-content").style.display = "block";
        },

        "_Gallery": function() {
        	var ac = etc.id("article-content");
            var imgs = etc.get("#article-content img");
            Gallery._Gallery.imgList = [];
            Gallery._Gallery.index = 0;

            for (var i = 0; i < imgs.length; i++) {
                var src = (imgs[i]).src;
                if (/thumbs\/\S+[0-9a-f]{38}\./.test(src)) {
                    Gallery._Gallery.imgList.push(src.replace("/thumbs/", "/images/"));
                } else {
                    Gallery._Gallery.imgList.push(src);
                }
            }

            if (Gallery._Gallery.imgList.length == 0) return;

            var html = "[<a href='javascript:Gallery._Gallery_Prev()'>上一张</a>]" +
                "<span class='gallery-pager'></span>" +
                "[<a href='javascript:Gallery._Gallery_Next()'>下一张</a>] " +
                "[<a href='javascript:Gallery._Gallery_Goto(true)'>重新载入</a>]" + 
                '<style>.gallery-page-no{margin:0 0.33em;text-decoration:none;}.gallery-page-no:before{content:"[";}.gallery-page-no:after{content:"]";}</style>';

            html = html +
                "<div><div id='gallery-loading' style='background-image:url(" + window.__cdn + "/assets/images/loading.gif); display: none; position: absolute; z-index:99; opacity: 0.5; filter: alpha(opacity=50);'></div><img onclick='Gallery._Gallery_Next()' id='gallery-image' style='cursor: pointer; max-width:100%; display: block'/></div>" + 
                html;

            // document.getElementById("article-content").innerHTML = html;
            var div = etc.id("gallery");
            if (div.id) {

            } else {
	            div = document.createElement("div");
	            div.innerHTML = html;
	            div.id = "gallery";
	            ac.parentNode.appendChild(div);
	        }

            ac.style.display = "none";
            div.style.display = "block";

            var pager = document.querySelectorAll(".gallery-pager");
            for (var i = 0; i < pager.length; i++) {
                var html = [];

                for (var j = 0; j < Gallery._Gallery.imgList.length; j++) {
                    html.push("<a id='gallery-page-" + i + "-" + j + "' class='gallery-page-no' href='javascript:Gallery._Gallery_Goto(" + j + ")'>" + (j + 1) + "</a>");
                }

                pager[i].innerHTML = html.join('');
            }

            Gallery._Gallery_Goto(0);
        },

        "_Gallery_Next": function() {
            Gallery._Gallery.index++;
            Gallery._Gallery_Goto(Gallery._Gallery.index);
        },

        "_Gallery_Prev": function() {
            Gallery._Gallery.index--;
            Gallery._Gallery_Goto(Gallery._Gallery.index);
        },

        "_Gallery_Goto": function(p) {
            if (p === true) p = Gallery._Gallery.index;

            if (p < 0)
            	p = 0;
           	else if (p >= Gallery._Gallery.imgList.length) 
           		p = Gallery._Gallery.imgList.length - 1;

           	Gallery._Gallery.index = p;

            for (var j = 0; j < Gallery._Gallery.imgList.length; j++) {
                var e1 = etc.id("gallery-page-1-" + j);
                var e2 = etc.id("gallery-page-0-" + j);
                e1.style.color = e2.style.color = "#ccc";
                e1.style.display = e2.style.display = (Math.abs(j - p) <= 5) ? "inherit" : "none";
            }

			var img = new Image();
            var i = etc.id("gallery-image");
            var loading = etc.id("gallery-loading");
			var oldTop = document.documentElement.scrollTop;

			img.onload = function(){
                etc.let.hide("gallery-loading");
                i.src = this.src; 
                etc.id("gallery-page-1-" + p).style.color = "black";
                etc.id("gallery-page-0-" + p).style.color = "black";
                document.documentElement.scrollTop = oldTop;
			};

			img.src = (Gallery._Gallery.imgList[p]);
			// i.src = window.__cdn + "/assets/images/loading.gif";
            loading.style.width = (i.clientWidth ? i.clientWidth : 64) + "px";
            loading.style.height = (i.clientHeight ? i.clientHeight : 64) + "px";
            
            if (img) etc.let.show("gallery-loading");
        }
    }
})();

var Animation = (function() {
    var handle;
    var __id;
    return {
        "start": function(id) {
            if (handle) {
                console.log("duplicated animation");
                return;
            }

            var __loadingSign = ["⣾","⣽","⣻","⢿","⡿","⣟","⣯","⣷"];
            var __index = 0;
            __id = id;
            handle = setInterval(function(){
                etc.id(id).innerHTML = __loadingSign[__index];
                __index++;
                if (__index > 7) __index = 0;
            }, 100);
        },

        "stop": function() {
            var e = etc.id(__id)
            if (e) e.innerHTML = "";

            if (handle) clearInterval(handle);
            handle = null;
        }
    };
})();