/* NicEdit - Micro Inline WYSIWYG
 * Copyright 2007-2008 Brian Kirchoff
 *
 * NicEdit is distributed under the terms of the MIT license
 * For more information visit http://nicedit.com/
 * Do not remove this copyright message
 */
/* This is a simplified version of NicEdit by Coyove
 * and it needs Helper object
 * coyove@hotmail.com
 */
var bkExtend = function() {
    var A = arguments;
    if (A.length == 1) {
        A = [this, A[0]]
    }
    for (var B in A[1]) {
        A[0][B] = A[1][B]
    }
    return A[0]
};

function bkClass() {}
bkClass.prototype.construct = function() {};
bkClass.extend = function(C) {
    var A = function() {
        if (arguments[0] !== bkClass) {
            return this.construct.apply(this, arguments)
        }
    };
    var B = new this(bkClass);
    bkExtend(B, C);
    A.prototype = B;
    A.extend = this.extend;
    return A
};
var bkElement = bkClass.extend({
    construct: function(B, A) {
        if (typeof(B) == "string") {
            B = (A || document).createElement(B)
        }
        B = $BK(B);
        return B
    },
    appendTo: function(A) {
        A.appendChild(this);
        return this
    },
    insertHTML: function(H) {
        this.innerHTML = H;
        return this
    },
    appendBefore: function(A) {
        A.parentNode.insertBefore(this, A);
        return this
    },
    addEvent: function(B, A) {
        bkLib.addEvent(this, B, A);
        return this
    },
    setContent: function(A) {
        this.innerHTML = A;
        return this
    },
    pos: function() {
        var C = curtop = 0;
        var B = obj = this;
        if (obj.offsetParent) {
            do {
                C += obj.offsetLeft;
                curtop += obj.offsetTop
            } while (obj = obj.offsetParent)
        }
        var A = (!window.opera) ? parseInt(this.getStyle("border-width") || this.style.border) || 0 : 0;
        return [C + A, curtop + A + this.offsetHeight]
    },
    noSelect: function() {
        bkLib.noSelect(this);
        return this
    },
    parentTag: function(A) {
        var B = this;
        do {
            if (B && B.nodeName && B.nodeName.toUpperCase() == A) {
                return B
            }
            B = B.parentNode
        } while (B);
        return false
    },
    hasClass: function(A) {
        return this.className.match(new RegExp("(\\s|^)nicEdit-" + A + "(\\s|$)"))
    },
    addClass: function(A) {
        if (!this.hasClass(A)) {
            this.className += " nicEdit-" + A
        }
        return this
    },
    removeClass: function(A) {
        if (this.hasClass(A)) {
            this.className = this.className.replace(new RegExp("(\\s|^)nicEdit-" + A + "(\\s|$)"), " ")
        }
        return this
    },
    setStyle: function(A) {
        var B = this.style;
        for (var C in A) {
            switch (C) {
                case "float":
                    B.cssFloat = B.styleFloat = A[C];
                    break;
                case "opacity":
                    B.opacity = A[C];
                    B.filter = "alpha(opacity=" + Math.round(A[C] * 100) + ")";
                    break;
                case "className":
                    this.className = A[C];
                    break;
                default:
                    B[C] = A[C]
            }
        }
        return this
    },
    getStyle: function(A, C) {
        var B = (!C) ? document.defaultView : C;
        if (this.nodeType == 1) {
            return (B && B.getComputedStyle) ? B.getComputedStyle(this, null).getPropertyValue(A) : this.currentStyle[bkLib.camelize(A)]
        }
    },
    remove: function() {
        this.parentNode.removeChild(this);
        return this
    },
    setAttributes: function(A) {
        for (var B in A) {
            this[B] = A[B]
        }
        return this
    }
});
var bkLib = {
    isMSIE: (navigator.appVersion.indexOf("MSIE") != -1),
    addEvent: function(C, B, A) {
        (C.addEventListener) ? C.addEventListener(B, A, false): C.attachEvent("on" + B, A)
    },
    toArray: function(C) {
        var B = C.length,
            A = new Array(B);
        while (B--) {
            A[B] = C[B]
        }
        return A
    },
    noSelect: function(B) {
        if (B.setAttribute && B.nodeName.toLowerCase() != "input" && B.nodeName.toLowerCase() != "textarea") {
            B.setAttribute("unselectable", "on")
        }
        for (var A = 0; A < B.childNodes.length; A++) {
            bkLib.noSelect(B.childNodes[A])
        }
    },
    camelize: function(A) {
        return A.replace(/\-(.)/g, function(B, C) {
            return C.toUpperCase()
        })
    },
    inArray: function(A, B) {
        return (bkLib.search(A, B) != null)
    },
    search: function(A, C) {
        for (var B = 0; B < A.length; B++) {
            if (A[B] == C) {
                return B
            }
        }
        return null
    },
    cancelEvent: function(A) {
        A = A || window.event;
        if (A.preventDefault && A.stopPropagation) {
            A.preventDefault();
            A.stopPropagation()
        }
        return false
    },
    domLoad: [],
    domLoaded: function() {
        if (arguments.callee.done) {
            return
        }
        arguments.callee.done = true;
        for (i = 0; i < bkLib.domLoad.length; i++) {
            bkLib.domLoad[i]()
        }
    },
    onDomLoaded: function(A) {
        this.domLoad.push(A);
        if (document.addEventListener) {
            document.addEventListener("DOMContentLoaded", bkLib.domLoaded, null)
        } else {
            if (bkLib.isMSIE) {
                document.write("<style>.nicEdit-main p { margin: 0; }</style><script id=__ie_onload defer " + ((location.protocol == "https:") ? "src='javascript:void(0)'" : "src=//0") + "><\/script>");
                $BK("__ie_onload").onreadystatechange = function() {
                    if (this.readyState == "complete") {
                        bkLib.domLoaded()
                    }
                }
            }
        }
        window.onload = bkLib.domLoaded
    }
};

function $BK(A) {
    if (typeof(A) == "string") {
        A = document.getElementById(A)
    }
    return (A && !A.appendTo) ? bkExtend(A, bkElement.prototype) : A
}
var bkEvent = {
    addEvent: function(A, B) {
        if (B) {
            this.eventList = this.eventList || {};
            this.eventList[A] = this.eventList[A] || [];
            this.eventList[A].push(B)
        }
        return this
    },
    fireEvent: function() {
        var A = bkLib.toArray(arguments),
            C = A.shift();
        if (this.eventList && this.eventList[C]) {
            for (var B = 0; B < this.eventList[C].length; B++) {
                this.eventList[C][B].apply(this, A)
            }
        }
    }
};

function __(A) {
    return A
}
Function.prototype.closure = function() {
    var A = this,
        B = bkLib.toArray(arguments),
        C = B.shift();
    return function() {
        if (typeof(bkLib) != "undefined") {
            return A.apply(C, B.concat(bkLib.toArray(arguments)))
        }
    }
};
Function.prototype.closureListener = function() {
    var A = this,
        C = bkLib.toArray(arguments),
        B = C.shift();
    return function(E) {
        E = E || window.event;
        if (E.target) {
            var D = E.target
        } else {
            var D = E.srcElement
        }
        return A.apply(B, [E, D].concat(C))
    }
};

var nicEditors = {
    nicPlugins: [],
    editors: [],
    registerPlugin: function(B, A) {
        this.nicPlugins.push({
            p: B,
            o: A
        })
    },
    allTextAreas: function(C) {
        var A = document.getElementsByTagName("textarea");
        for (var B = 0; B < A.length; B++) {
            nicEditors.editors.push(new nicEditor(C).panelInstance(A[B]))
        }
        return nicEditors.editors
    },
    findEditor: function(C) {
        var B = nicEditors.editors;
        for (var A = 0; A < B.length; A++) {
            if (B[A].instanceById(C)) {
                return B[A].instanceById(C)
            }
        }
    }
};
var nicEditor = bkClass.extend({
    construct: function(C) {
        // this.options = new nicEditorConfig();
        // bkExtend(this.options, C);
        this.nicInstances = new Array();
        this.loadedPlugins = new Array();
        bkLib.addEvent(document.body, "mousedown", this.selectCheck.closureListener(this))
    },
    panelInstance: function(B, C) {
        
        return this.addInstance(B, C)
    },
    addInstance: function(B, C) {
        var A = new nicEditorInstance($BK(B), C, this)
        this.nicInstances.push(A);
        return this
    },
    removeInstance: function(C) {
        C = $BK(C);
        var B = this.nicInstances;
        for (var A = 0; A < B.length; A++) {
            if (B[A].e == C) {
                B[A].remove();
                this.nicInstances.splice(A, 1)
            }
        }
    },
    removePanel: function(A) {
        if (this.nicPanel) {
            this.nicPanel.remove();
            this.nicPanel = null
        }
    },
    instanceById: function(C) {
        C = $BK(C);
        var B = this.nicInstances;
        for (var A = 0; A < B.length; A++) {
            if (B[A].e == C) {
                return B[A]
            }
        }
    },
    nicCommand: function(B, A) {
        if (this.selectedInstance) {
            this.selectedInstance.nicCommand(B, A)
        }
    },
    selectCheck: function(C, A) {
        var B = false;
        do {
            if (A.className && A.className.indexOf("nicEdit") != -1) {
                return false
            }
        } while (A = A.parentNode);
        this.fireEvent("blur", this.selectedInstance, A);
        this.lastSelectedInstance = this.selectedInstance;
        this.selectedInstance = null;
        return false
    }
});
nicEditor = nicEditor.extend(bkEvent);
var nicEditorInstance = bkClass.extend({
    isSelected: false,
    construct: function(G, D, C) {
        this.ne = C;
        this.elm = this.e = G;
        this.options = null;
        newX = parseInt(G.getStyle("width")) || G.clientWidth;
        newY = parseInt(G.getStyle("height")) || G.clientHeight;
        this.initialHeight = newY - 8;
        var H = (G.nodeName.toLowerCase() == "textarea");
        if (H) {
            var B = (bkLib.isMSIE && !((typeof document.body.style.maxHeight != "undefined") && document.compatMode == "CSS1Compat"));
            var E = {
                // width: newX + "px",
                width: "100%",
                border: "1px solid #ccc",
                borderTop: 0,
                overflowY: "auto",
                overflowX: "hidden",
                boxShadow: "inset 0 1px 2px rgba(0,0,0,0.15)",
                backgroundColor: "white",
                textAlign: "left"
            };

            this.editorContain = new bkElement("DIV").setStyle(E).appendBefore(G);
            var A = new bkElement("DIV").setStyle({
                width: "100%",
                padding: "4px",
                boxSizing: "border-box",
                minHeight: newY + "px"
            }).appendTo(this.editorContain);
            G.setStyle({
                display: "none"
            });
            A.innerHTML = G.innerHTML;
            A.setAttribute("id", "cyv-main-editor");
            if (H) {
                A.setContent(G.value);
                this.copyElm = G;
                var F = G.parentTag("FORM");
            }
            A.setStyle({
                overflow: "hidden"
            });
            A.setAttribute("tabindex", "0");
            this.elm = A
        }
        this.ne.addEvent("blur", this.blur.closure(this));
        this.init();
        this.blur()
    },
    init: function() {
        this.elm.setAttribute("contentEditable", "true");
        if (this.getContent() == "") {
            this.setContent("<br />")
        }
        this.instanceDoc = document.defaultView;
        this.elm.addEvent("mousedown", this.selected.closureListener(this)).addEvent("keypress", this.keyDown.closureListener(this)).addEvent("focus", this.selected.closure(this)).addEvent("blur", this.blur.closure(this)).addEvent("keyup", this.selected.closure(this));
        this.ne.fireEvent("add", this)
    },
    remove: function() {
        this.saveContent();
        if (this.copyElm || this.options.hasPanel) {
            this.editorContain.remove();
            this.e.setStyle({
                display: "block"
            });
            this.ne.removePanel()
        }
        this.disable();
        this.ne.fireEvent("remove", this)
    },
    disable: function() {
        this.elm.setAttribute("contentEditable", "false")
    },
    getSel: function() {
        return (window.getSelection) ? window.getSelection() : document.selection
    },
    getRng: function() {
        var A = this.getSel();
        if (!A || A.rangeCount === 0) {
            return
        }
        return (A.rangeCount > 0) ? A.getRangeAt(0) : A.createRange()
    },
    selRng: function(A, B) {
        if (window.getSelection) {
            B.removeAllRanges();
            B.addRange(A)
        } else {
            A.select()
        }
    },
    selElm: function() {
        var C = this.getRng();
        if (!C) {
            return
        }
        if (C.startContainer) {
            var D = C.startContainer;
            if (C.cloneContents().childNodes.length == 1) {
                for (var B = 0; B < D.childNodes.length; B++) {
                    var A = D.childNodes[B].ownerDocument.createRange();
                    A.selectNode(D.childNodes[B]);
                    if (C.compareBoundaryPoints(Range.START_TO_START, A) != 1 && C.compareBoundaryPoints(Range.END_TO_END, A) != -1) {
                        return $BK(D.childNodes[B])
                    }
                }
            }
            return $BK(D)
        } else {
            return $BK((this.getSel().type == "Control") ? C.item(0) : C.parentElement())
        }
    },
    saveRng: function() {
        this.savedRange = this.getRng();
        this.savedSel = this.getSel()
    },
    restoreRng: function() {
        if (this.savedRange) {
            this.selRng(this.savedRange, this.savedSel)
        }
    },
    keyDown: function(B, A) {
        if (B.ctrlKey) {
            this.ne.fireEvent("key", this, B)
        }
    },
    selected: function(C, A) {
        if (!A && !(A = this.selElm)) {
            A = this.selElm()
        }
        if (!C.ctrlKey) {
            var B = this.ne.selectedInstance;
            if (B != this) {
                if (B) {
                    this.ne.fireEvent("blur", B, A)
                }
                this.ne.selectedInstance = this;
                this.ne.fireEvent("focus", B, A)
            }
            this.ne.fireEvent("selected", B, A);
            this.isFocused = true;
            this.elm.addClass("selected")
        }
        return false
    },
    blur: function() {
        this.isFocused = false;
        this.elm.removeClass("selected")
    },
    saveContent: function() {
        if (this.copyElm || this.options.hasPanel) {
            this.ne.fireEvent("save", this);
            (this.copyElm) ? this.copyElm.value = this.getContent(): this.e.innerHTML = this.getContent()
        }
    },
    getElm: function() {
        return this.elm
    },
    getContent: function() {
        this.content = this.getElm().innerHTML;
        this.ne.fireEvent("get", this);
        return this.content
    },
    setContent: function(A) {
        this.content = A;
        this.ne.fireEvent("set", this);
        this.elm.innerHTML = this.content;
    },
});