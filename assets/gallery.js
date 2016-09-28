var Gallery = (function() {
    return {
        "_Normal": function() {
        	etc.id("gallery-content").style.display = "none";
            etc.id("article-content").style.display = "block";

            var doms = Gallery._Gallery.imgDOMs;
            for (var i = 0; i < doms.length; i++) {
                doms[i][0].src = doms[i][1];
            }
        },

        "_Gallery": function() {
        	var ac = etc.id("article-content");
            var imgs = etc.get("#article-content img");
            var links = etc.get("#article-content a");
            var div = etc.id("gallery-content");
            ac.style.display = "none";

            if (div.id) {
                div.style.display = "block";
                Gallery._Gallery_Goto(0);
                return;
            }

            Gallery._Gallery.imgList = [];
            Gallery._Gallery.imgDOMs = [];
            Gallery._Gallery.index = 0;

            var ifLarger = {};
            for (var i = 0; i < links.length; i++) {
                var m = links[i].href.match(/img\.tmp\.is\/(\S+)/);
                if (m) {
                    ifLarger[m[1]] = links[i].href;
                } else {
                    m = links[i].href.match(/images\/(\S+)/);
                    if (m) ifLarger[m[1]] = links[i].href;
                }
            }

            for (var i = 0; i < imgs.length; i++) {
                var isrc = imgs[i].src ? imgs[i].src : imgs[i].getAttribute("data-src");
                Gallery._Gallery.imgDOMs.push([imgs[i], imgs[i].src]);

                var m = isrc.match(/\/small-(\S+)/);
                if (m && ifLarger[m[1]]) {
                    Gallery._Gallery.imgList.push(ifLarger[m[1]]);
                } else {
                    m = isrc.match(/thumbs\/(\S+)/);
                    if (m && ifLarger[m[1]]) {
                        Gallery._Gallery.imgList.push(ifLarger[m[1]]);
                    } else {
                        Gallery._Gallery.imgList.push(isrc);
                    }
                }

                imgs[i].src = "about:blank";
            }
            

            div = document.createElement("div");
            div.id = "gallery-content";
            ac.parentNode.appendChild(div);
            div.style.display = "block";

            var paging = [
                "<style>",
                "#gallery-image { transform-origin: top left; -webkit-transform-origin: top left; -ms-transform-origin: top left;}",
                "#gallery-container.r90 img {",
                    "transform: rotate(90deg) translateY(-100%);",
                    "-webkit-transform: rotate(90deg) translateY(-100%);",
                    "-ms-transform: rotate(90deg) translateY(-100%);",
                "}",
                "#gallery-container.r180 img {",
                    "transform: rotate(180deg) translate(-100%, -100%);",
                    "-webkit-transform: rotate(180deg) translate(-100%, -100%);",
                    "-ms-transform: rotate(180deg) translateX(-100%, -100%);",
                "}",
                "#gallery-container.r270 img {",
                    "transform: rotate(270deg) translateX(-100%);",
                    "-webkit-transform: rotate(270deg) translateX(-100%);",
                    "-ms-transform: rotate(270deg) translateX(-100%);",
                "}",
                "</style>",
                "<table class='pager'>",
                    "<td class='c'><a href='javascript:Gallery._Gallery_Prev()'>",
                    "<span class='fai'>&nbsp;&#xe046;&nbsp;</span></a></td>",
                    "<td class='nc'><select class='gallery-pager'></select></td>",
                    "<td class='c'><a href='javascript:Gallery._Gallery_Next()'>",
                    "<span class='fai'>&nbsp;&#xe048;&nbsp;</span></a></td>",
                    "<td class='c'><a href='javascript:Gallery._Gallery_Goto(true)'>",
                    "<span class='fai'>&nbsp;&#xe01f;&nbsp;</span></a></td>",
                "</table>",
            ].join('');                

            div.innerHTML = paging + [
                    "<div id='gallery-container'>",
                        "<div style='",
                            "position: absolute;",
                            "z-index: 100;",
                            "font-size: 150%;",
                            "margin: 10px;",
                            "border: solid 1px;",
                            "line-height: 1;",
                            "padding: 4px 8px;",
                            "background: white;",
                            "opacity: 0.75;'>",
                            "<a class='none' href='javascript:Gallery._Gallery_Rotate()'><span class='fai'>&#xe0d9;</span></a>",
                        "</div>",
                        "<div id='gallery-loading' style='",
                            "background-image:url(" + window.__cdn + "/assets/images/loading.gif);",
                            "display: none;",
                            "position: absolute;", 
                            "z-index: 99;",
                            "opacity: 0.5;",
                            "filter: alpha(opacity=50);'>",
                        "</div>",
                        "<img onclick='Gallery._Gallery_Next()' id='gallery-image' style='",
                            "cursor: pointer;",
                            "max-width: 100%;",
                            "display: block'/>",
                    "</div>"].join('') + paging;

            var pager = etc.get(".gallery-pager");
            for (var i = 0; i < pager.length; i++) {
                var html = [];

                for (var j = 0; j < Gallery._Gallery.imgList.length; j++) {
                    html.push("<option value='" + j + "'>&nbsp;" + (j + 1) + "&nbsp;</option>");
                }

                pager[i].innerHTML = html.join('');
                pager[i].onchange = function() {
                    Gallery._Gallery_Goto(this.value);
                }
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
            if (Gallery._Gallery.imgList.length == 0) return;
            if (p === true) p = Gallery._Gallery.index;

            if (p < 0)
            	p = 0;
           	else if (p >= Gallery._Gallery.imgList.length) 
           		p = Gallery._Gallery.imgList.length - 1;

           	Gallery._Gallery.index = p;

			var img = new Image();
            var gi = etc.id("gallery-image");
            var loading = etc.id("gallery-loading");
			var oldTop = document.documentElement.scrollTop;

            var gc = etc.id("gallery-container");
            gc.style.width = "";
            gc.style.height = "";

			img.onload = function() {
                etc.let.hide("gallery-loading");
                gi.src = this.src; 

                var pager = etc.get(".gallery-pager");
                for (var i = 0; i < pager.length; i++) pager[i].value = p;

                document.documentElement.scrollTop = oldTop;
                Gallery._Gallery_Rotate(true);
			};

			img.src = (Gallery._Gallery.imgList[p]);
			// i.src = window.__cdn + "/assets/images/loading.gif";
            loading.style.width = (gi.clientWidth ? gi.clientWidth : 64) + "px";
            loading.style.height = (gi.clientHeight ? gi.clientHeight : 64) + "px";

            if (img) etc.let.show("gallery-loading");
        },

        "_Gallery_Rotate": function(init) {
            var gi = etc.id("gallery-image");
            var gc = etc.id("gallery-container");
            var gwidth = gi.clientWidth;
            var gheight = gi.clientHeight;
            var deg;

            if (init) {
                deg = 0;
            } else {
                deg = parseInt(gc.attr("data-deg"));
                deg = (deg + 90) % 360;
            }

            gc.attr("data-deg", deg);

            if (deg == 0) {
                gc.className = "";
            } else {
                gc.className = "r" + deg;
            }

            if (deg == 0 || deg == 180) {
                gc.style.width = gwidth + "px";
                gc.style.height = gheight + "px";
            } else {
                gc.style.width = gheight + "px";
                gc.style.height = gwidth + "px";
            }
        }
    }
})();