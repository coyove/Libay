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

    String.prototype.score=function(e,f){if(this===e)return 1;if(""===e)return 0;var d=0,a,g=this.toLowerCase(),n=this.length,h=e.toLowerCase(),k=e.length,b;a=0;var l=1,m,c;f&&(m=1-f);if(f)for(c=0;c<k;c+=1)b=g.indexOf(h[c],a),-1===b?l+=m:(a===b?a=.7:(a=.1," "===this[b-1]&&(a+=.8)),this[b]===e[c]&&(a+=.1),d+=a,a=b+1);else for(c=0;c<k;c+=1){b=g.indexOf(h[c],a);if(-1===b)return 0;a===b?a=.7:(a=.1," "===this[b-1]&&(a+=.8));this[b]===e[c]&&(a+=.1);d+=a;a=b+1}d=.5*(d/n+d/k)/l;h[0]===g[0]&&.85>d&&(d+=.15);return d};

    // http://stackoverflow.com/questions/2308134/trim-in-javascript-not-working-in-ie
    if (typeof String.prototype.trim !== 'function') {
        String.prototype.trim = function() {
            return this.replace(/^\s+|\s+$/g, ''); 
        }
    }

    if (typeof Array.prototype.forEach !== 'function') {
        Array.prototype.forEach= function(action, that /*opt*/) {
            for (var i= 0, n= this.length; i<n; i++)
                if (i in this) {
                    var b = action.call(that, this[i], i, this);
                    if (b) break;
                }
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
            else
                e.innerHTML = html;

            return e;
        }
        e.attr = function(a, b) {
            if (typeof b !== 'undefined') {
                e.setAttribute(a, b);
                return e;
            } else {
                return e.getAttribute(a);
            }
        }

        return e;
    };

    function _WaitObject() { this.isDone = false; }
    _WaitObject.prototype._call = function() { 
        this.callback();
        if (this.then_callback) this.then_callback();

        this.isDone = false; 
        this.callback = null; 
        this.then_callback = null;
    }

    _WaitObject.prototype.done = function() { 
        this.isDone = true; 
        if (this.callback) this._call(); 
        return this; 
    }

    _WaitObject.prototype.wait = function(callback) { 
        this.callback = callback; 
        if (this.isDone) this._call(); 
        return this; 
    }

    _WaitObject.prototype.then = function(then_callback) { 
        this.then_callback = then_callback; 
        if (this.isDone) this._call(); 
        return this; 
    }

    g.etc = {
        "onload": function(func) {
            if (document.addEventListener) 
                document.addEventListener("DOMContentLoaded", func, false);
            else
                window.attachEvent("onload", func);
        },

        "id": _id,

        "body": function() {
            return document.getElementsByTagName('body')[0];
        },

        "width": function(dom) {
            if (dom) {
                return dom.offsetWidth;
            } else {
                return window.innerWidth || document.documentElement.clientWidth || document.body.clientWidth;
            }
        },

        "height": function(dom) {
            if (dom) {
                return dom.offsetHeight;
            } else {
                return window.innerHeight || document.documentElement.clientHeight || document.body.clientHeight;
            }
        },

        "coord": function(event) {
            var dot, eventDoc, doc, body, pageX, pageY;

            event = event || window.event; // IE-ism
            if (event.pageX == null && event.clientX != null) {
                eventDoc = (event.target && event.target.ownerDocument) || document;
                doc = eventDoc.documentElement;
                body = eventDoc.body;

                event.pageX = event.clientX +
                  (doc && doc.scrollLeft || body && body.scrollLeft || 0) -
                  (doc && doc.clientLeft || body && body.clientLeft || 0);
                event.pageY = event.clientY +
                  (doc && doc.scrollTop  || body && body.scrollTop  || 0) -
                  (doc && doc.clientTop  || body && body.clientTop  || 0 );
            }

            return event;
        },

        "wait": {
            "on": function(n) { 
                if (typeof n === "String" && n.test(/(on|onclick)/)) throw "Invalid name";

                g.etc.wait[n] = g.etc.wait[n] || new _WaitObject; 
                return g.etc.wait[n]; 
            },

            "onclick": function(e) {
                if (e.getAttribute("data-disabled") === "true") return;

                e.disabled = true;
                e.setAttribute("data-disabled", "true");

                var __overlay = document.createElement("div");
                __overlay.style.position = "fixed";
                __overlay.style.width = __overlay.style.height = "100%";
                __overlay.style.left = __overlay.style.top = "0px";
                __overlay.style.zIndex = "65535";
                __overlay.style.cursor = "wait";

                var __body = document.getElementsByTagName("body")[0];
                __body.appendChild(__overlay);

                var __func = e.getAttribute("data-onclick");
                var __html = e.innerHTML;
                var __index = 0;
                var __handle = setInterval(function(){ e.innerHTML = "⠇⠋⠙⠸⠴⠦"[__index++ % 6]; }, 100);

                eval(__func).wait(function() {
                    clearInterval(__handle);
                    e.innerHTML = __html;
                    e.disabled = false;
                    e.setAttribute("data-disabled", "false");

                    __body.removeChild(__overlay);
                });
            },
        },

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

        "get": function(selector) { 
            var doms = document.querySelectorAll(selector); 
            if (doms.forEach) {} else { doms.forEach = Array.prototype.forEach; }
            return doms;
        },

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
                case "li":
                    e.style.display = "list-item";
                    break;
                case "a":
                case "span":
                    e.style.display = "inherit";
                    break;
                case "button":
                    e.style.display = "inline-block";
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
            "format": function(timestamp, seconds) {
                var d = new Date(timestamp);
                var today = new Date();
                var yyyy = d.getFullYear();
                var mm = d.getMonth() < 9 ? "0" + (d.getMonth() + 1) : (d.getMonth() + 1);
                var dd = d.getDate() < 10 ? "0" + d.getDate() : d.getDate();
                var hh = d.getHours() < 10 ? "0" + d.getHours() : d.getHours();
                var min = d.getMinutes() < 10 ? "0" + d.getMinutes() : d.getMinutes();
                var ss = d.getSeconds() < 10 ? "0" + d.getSeconds() : d.getSeconds();

                ret = yyyy + "/" + mm + "/" + dd + " " + hh + ":" + min + (seconds ? ":" + ss : "");
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
            "escape": function(str) {
                return str
                     .replace(/&/g, "&amp;")
                     .replace(/</g, "&lt;")
                     .replace(/>/g, "&gt;")
                     .replace(/"/g, "&quot;")
                     .replace(/'/g, "&#039;");
            },

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

            "guid": function() {
                function s4() {
                    return Math.floor((1 + Math.random()) * 0x10000)
                        .toString(16)
                        .substring(1);
                }
                return s4() + s4() + '-' + s4() + '-' + s4() + '-' +
                    s4() + '-' + s4() + s4() + s4();
            },

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
                        
                        for(var i in scripts) {
                            if (scripts[i].src == _src) {
                                _gotoNext();
                                return;
                            }
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

            "cookie": {
                "read": function (cname) {
                    var name = cname + "=";
                    var ca = document.cookie.split(';');
                    for(var i = 0; i <ca.length; i++) {
                        var c = ca[i];
                        while (c.charAt(0)==' ') {
                            c = c.substring(1);
                        }
                        if (c.indexOf(name) == 0) {
                            return c.substring(name.length,c.length);
                        }
                    }
                    return "";
                },

                "write": function (cname, cvalue, exp) {
                    var d = new Date();
                    d.setTime(d.getTime() + exp);

                    document.cookie = cname + "=" + cvalue + "; expires=" + d.toUTCString() + "; path=/;";
                },

                "delete": function(cname) {
                    g.etc.cookie.remove(cname);
                },

                "remove": function(cname) {
                    document.cookie = cname + "=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
                }
            },
        },
        
        "editor": {
            "insertText": function(myField, myValue) {
                //IE support
                if (document.selection) {
                    myField.focus();
                    sel = document.selection.createRange();
                    sel.text = myValue;
                }
                //MOZILLA and others
                else if (myField.selectionStart || myField.selectionStart == '0') {
                    var startPos = myField.selectionStart;
                    var endPos = myField.selectionEnd;
                    myField.value = myField.value.substring(0, startPos)
                        + myValue
                        + myField.value.substring(endPos, myField.value.length);
                    myField.selectionStart = startPos + myValue.length;
                    myField.selectionEnd = startPos + myValue.length;
                }
            },

            "uploadImage": function(files, callback, options) {
                options = options || {};

                if (files.length == 0) {
                    callback();
                    return;
                }

                var file = files.shift();

                var onError = function(A) {
                    if (callback) callback("Err::Upload::" + A, file);
                };

                var payload = { "image": file };
                if (options["additional_form"]) {
                    for (var k in options["additional_form"])
                        payload[k] = options["additional_form"][k];
                }
 
                etc.util.ajax.$post(options["type"] == "imgur" ? 
                    "https://api.imgur.com/3/image" : "/upload",
                    payload, {
                    'Authorization': 'Client-ID c37fc05199a05b7'
                }).then(function(e, rt, x) {
                    if (e) {
                        onError("Network");
                    } else {
                        try {
                            var D = options["type"] == "imgur" ? JSON.parse(rt).data : JSON.parse(rt);
                        } catch (E) {
                            onError(rt == "Err::CSRF::CSRF_Failure" ? "CSRF_Failure" : "Invalid_JSON");
                            return;
                        }

                        if (D.Error || D.error) {
                            onError(D.R);
                        } else {
                            var _link = (D.Link || D.link).replace("http://", "https://");
                            var _thumb = (D.Thumbnail || D.link).replace("http://", "https://");

                            if (options["editor"]) {
                                etc.editor.insertText(_id(options["editor"]), 
                                    "[url=_blank;" + _link + "][img]" + _thumb + "[/img][/url]");
                            }

                            options["uploaded"] = options["uploaded"] || [];
                            options["uploaded"].push([_thumb, _link]);

                            if (options["uploaded_callback"])
                                options["uploaded_callback"](_thumb, _link);
                        }
                    }

                    etc.editor.uploadImage(files, callback, options);
                });
            }
        },
    };

    g.etc.onload(function() {
        var elems = document.querySelectorAll("*[data-onclick]");
        for (var i = 0; i < elems.length; i++) {
            if (elems[i].tagName === "A") elems[i].href = "javascript:void(0)";

            elems[i].onclick = (function(idx){
                return function() { g.etc.wait.onclick(elems[idx]); };
            })(i);
        }
  
        var ddelems = document.querySelectorAll("*[data-dropdown]");
        for (var i = 0; i < ddelems.length; i++) {
            ddelems[i].style.cursor = "pointer";
            var dummy = (function(idx){
                return function() { 
                    var el = ddelems[idx];
                    var dd = g.etc.id(el.getAttribute("data-dropdown"));
                    var ddol = el.getAttribute("data-dropdown-onload");
                    var sub = el.getAttribute("data-subdropdown"); 
                    var rect = el.getBoundingClientRect();
                    if (el.children && el.children[0] && sub != "true")
                        rect = el.children[0].getBoundingClientRect();

                    if (sub == "true") {
                        dd.style.left = (rect.left) + "px";
                        dd.style.top = (rect.top) + "px";
                    } else {
                        dd.style.left = (rect.left - 10) + "px";
                        dd.style.top = (rect.bottom + 10) + "px";
                    }

                    dd.style.display = "block";
                    dd.style.zIndex = 99;

                    var underlay = document.createElement("div");
                    underlay.className = "dropdown-underlay";
                    underlay.style.zIndex = 98;
                    underlay.style.display = "block";

                    dd.parentNode.insertBefore(underlay, dd);
                    var body = document.getElementsByTagName('body')[0];
                    body.className += " stop-scrolling";

                    underlay.onclick = dd.onclick = function() {
                        dd.style.display = "none";
                        dd.parentNode.removeChild(underlay);
                        body.className = body.className.replace(" stop-scrolling", "");
                        el.focus();
                    };

                    var inputs = dd.querySelectorAll("input");
                    for (var i = 0; i < inputs.length; i++) {
                        if (inputs[i].type == "checkbox") continue;
                        
                        inputs[i].onclick = function(e) {
                            if (!e) e = window.event;
                            
                            e.cancelBubble = true;
                            e.stopPropagation();
                        }
                    }

                    if (ddol) {
                        eval(ddol);
                    } else if (el.onload) {
                        el.onload();
                    }
                };
            })(i);

            if (ddelems[i].getAttribute("data-dblclick-dropdown") == "true")
                ddelems[i].ondblclick = dummy;
            else
                ddelems[i].onclick = dummy;
        }

        var menu = document.querySelector(".dropdown.contextmenu");
        if (menu) g.etc.body().oncontextmenu = function (event) {
            event.preventDefault();
            event = g.etc.coord(event);

            menu.style.left = event.pageX + "px";

            if (event.pageX + 200 > etc.width()) {
                menu.style.left = (event.pageX - 200) + "px";
            }

            menu.style.top = event.pageY + "px";
            menu.style.display = "block";
            menu.style.zIndex = 99;

            etc.body().onclick = menu.onclick = function() {
                menu.style.display = "none";
                etc.body().onclick = null;
            };

            var inputs = menu.querySelectorAll("input");
            for (var i = 0; i < inputs.length; i++) {
                if (inputs[i].type == "checkbox") continue;
                
                inputs[i].onclick = function(e) {
                    if (!e) e = window.event;
                    
                    e.cancelBubble = true;
                    e.stopPropagation();
                }
            }
        }
    });
    
})(this);

var Animation = (function() {
    var handle;
    var __id;
    return {
        "start": function(id) {
            if (handle) return;

            var __index = 0;
            __id = id;

            handle = setInterval(function(){ etc.id(id).innerHTML = "⠇⠋⠙⠸⠴⠦"[__index++ % 6]; }, 100);
        },

        "stop": function() {
            var e = etc.id(__id)
            if (e) e.innerHTML = "";

            if (handle) clearInterval(handle);
            handle = null;
        }
    };
})();

etc.onload(function() {
    var kd = etc.get(".dropdown-normal");
    var appendKeyword = function(input, word) {
        if (input.value == "") {
            input.value = word + ' ';
        } else {
            var tmp = input.value.split(" ");
            for (var i = 0; i < tmp.length; i++) {
                if (tmp[i] == "") {         
                    tmp.splice(i, 1);
                    i--;
                }
            }

            if (input.value.charAt(input.value.length - 1) == ' ') {
                tmp.push(word + ' ');
            } else {
                tmp = tmp.splice(0, tmp.length - 1);
                tmp.push(word + ' ');
            }

            input.value = tmp.join(' ');
        }

        input.focus();
    }

    kd.forEach(function(e) {
        var input = e.getElementsByTagName('input')[0];

        etc.id(input).on("keyup", (function(dd) {
            return function (ev) {
                var keyCode = ev.keyCode || ev.which;
                if ([9, 13, 32, 37, 38, 39, 40].indexOf(keyCode) == -1) {
                    if (window.__keywords) {} else {
                        etc.util.ajax.get("/get/keywords").then(function (e, list) {
                            window.__keywords = JSON.parse(list);
                        });
                        return;
                    }

                    var cands = [];
                    var tester = this.value.split(' ');
                    for (var k in window.__keywords) {
                        if (k != "") 
                            cands.push([k.score(tester[tester.length - 1]), k, window.__keywords[k] + 1]);
                    }

                    cands = cands.sort(function(a, b) { 
                        return b[0] * b[2] * 10 - a[0] * a[2] * 10; 
                    }).splice(0, 10).sort(function(a, b) { 
                        return b[0] - a[0];
                    });

                    var ul = document.createElement("ul");
                    ul.setAttribute("data-current", -1);
                    ul.setAttribute("data-length", cands.length);

                    var userInput = this;
                    for (var i = 0; i < cands.length; i++) {
                        var li = document.createElement("li");
                        li.innerHTML = etc.string.escape(cands[i][1]) + " (" + cands[i][2] + ")";
                        li.onclick = (function(kw) {
                            return function() {
                                appendKeyword(userInput, kw);
                                dd.removeChild(ul);
                            }
                        } (cands[i][1]));

                        ul.appendChild(li);
                    }

                    if (dd.children.length > 1)
                        dd.removeChild(dd.children[1]);

                    dd.appendChild(ul);
                }

                if (this.value == "") {
                    var ul = dd.getElementsByTagName('ul')[0];
                    ul && dd.removeChild(ul);
                }
            }
        } (e))).on("keydown", (function(dd) {
            return function (ev) {
                var highlightLi = function(ul, cur) {
                    var lis = ul.getElementsByTagName('li');
                    for (var i = 0; i < lis.length; i++) {
                        lis[i].setAttribute("selected", i == cur ? "selected" : "");
                    }
                }; 

                if (ev.keyCode == 9 || ev.which == 9) {
                    ev.preventDefault();
                    ev.stopPropagation();

                    var lis = dd.getElementsByTagName('ul')[0].getElementsByTagName('li');
                    if (lis[0]) lis[0].click();
                } else if (ev.keyCode == 13 || ev.which == 13) {
                    var lis = dd.getElementsByTagName('ul')[0].getElementsByTagName('li');
                    for (var i = 0; i < lis.length; i++) {
                        if (lis[i].getAttribute("selected") == "selected") {
                            lis[i].click();
                            return;
                        }
                    }
                    dd.removeChild(dd.getElementsByTagName('ul')[0]);
                } else if (ev.keyCode == 38 || ev.which == 38) {
                    var ul = etc.id(dd.getElementsByTagName('ul')[0]);
                    var cur = parseInt(ul.attr("data-current")) - 1;
                    if (cur < 0) cur = parseInt(ul.attr("data-length")) - 1;

                    ul.attr("data-current", cur);
                    highlightLi(ul, cur);
                    
                    ev.preventDefault();
                    ev.stopPropagation();
                } else if (ev.keyCode == 40 || ev.which == 40) {
                    var ul = etc.id(dd.getElementsByTagName('ul')[0]);
                    var cur = parseInt(ul.attr("data-current")) + 1;
                    if (cur > parseInt(ul.attr("data-length")) - 1) cur = 0;

                    ul.attr("data-current", cur);
                    highlightLi(ul, cur);
                    
                    ev.preventDefault();
                    ev.stopPropagation();
                } else if ([32, 37, 39].indexOf(ev.keyCode) > -1 || 
                    [32, 37, 39].indexOf(ev.which) > -1 || this.value == "") {
                    var ul = dd.getElementsByTagName('ul')[0];
                    ul && dd.removeChild(ul);
                }
            }
        } (e)));
    });
});