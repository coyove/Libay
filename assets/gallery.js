var Gallery = (function() {
    var generateList = function(length, max, seed) {
        var ret = [];
        var i = 0;
        while (i < length) {
            seed = (2097151*seed + 13739) % 4294967296;
            ret.push(parseInt(seed / 4294967296 * max));
            i++;
        }

        return ret;
    }

    var block = 32;
    var isLoading = false;
    var password = 0xc0ffee;

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
                    Gallery._Gallery.imgList.push(isrc);
                }

                imgs[i].src = "about:blank";
            }
            

            div = document.createElement("div");
            div.id = "gallery-content";
            ac.parentNode.insertBefore(div, ac);
            div.style.display = "block";

            var paging = [
                "<table class='pager tox'>",
                    "<td class='c'><a href='javascript:Gallery._Gallery_Prev()'>",
                    "<span class='fai'>&#xe00e;</span></a></td>",
                    "<td class='nc'><select class='gallery-pager'></select>",
                    "<a href='javascript:Gallery._Gallery_Goto(true)'>",
                    "<span class='fai' style='font-size:120%'>&#xe0da;</span></a></td>",
                    "<td class='c'><a href='javascript:Gallery._Gallery_Next()'>",
                    "<span class='fai'>&#xe01b;</span></a></td>",
                    "<td class='c'><a href='javascript:Gallery._Size(0)'>",
                    "<span class='fai'>&#xe08d;</span></a></td>",
                    "<td class='c'><a href='javascript:Gallery._Size(1)'>",
                    "<span class='fai' style='font-size:120%;'>&#xe08d;</span></a></td>",
                    "<td class='c'><a href='javascript:Gallery._Size(2)'>",
                    "<span class='fai' style='font-size:150%;'>&#xe08d;</span></a></td>",
                "</table>",
            ].join('\n');                

            div.innerHTML = paging + [
                    "<style>",
                        ".tox .fai { padding: 0 6px }",
                        ".image-large {",
                            "max-width: 1024px;",
                        "}",
                        ".image-medium {",
                            "max-width: 800px;",
                        "}",
                        ".image-small {",
                            "max-width: 600px;",
                        "}",
                    "</style>",
                    "<div id='gallery-container'>",
                        "<div id='gallery-loading' style='",
                            "background-image:url(" + window.__cdn + "/assets/images/loading.gif);",
                            "display: none;",
                            "position: absolute;", 
                            "z-index: 89;",
                            "opacity: 0.5;",
                            "filter: alpha(opacity=50);'>",
                        "</div>",
                        "<canvas onclick='Gallery._Gallery_Next()' id='gallery-image' class='image-large' style='",
                            "cursor: pointer;",
                            "width: 100%;",
                            "display: block'></canvas>",
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
            var ups = etc.util.cookie.read("ups");
            if (ups) {
                Gallery._Size(parseInt(ups));
            }
        },

        "_Size": function(level) {
            etc.id("gallery-image").className = "image-" + ["small", "medium", "large"][level];
            etc.util.cookie.write("ups", level);
        },

        "_Gallery_Next": function() {
            Gallery._Gallery_Goto(++Gallery._Gallery.index);
        },

        "_Gallery_Prev": function() {
            Gallery._Gallery_Goto(--Gallery._Gallery.index);
        },

        "_Gallery_Goto": function(p) {
            if (Gallery._Gallery.imgList.length == 0) return;
            if (isLoading) {
                console.log(1);
                return;
            }
            isLoading = true;
            if (p === true) p = Gallery._Gallery.index;

            if (p < 0)
                p = 0;
            else if (p >= Gallery._Gallery.imgList.length) 
                p = Gallery._Gallery.imgList.length - 1;

            Gallery._Gallery.index = p;

            var img = new Image();
            var gi = etc.id("gallery-image");
            var ctx = gi.getContext('2d');
            var loading = etc.id("gallery-loading");
            var oldTop = document.documentElement.scrollTop;

            var gc = etc.id("gallery-container");
            gc.style.width = "";
            gc.style.height = "";

            img.onload = function() {
                etc.let.hide("gallery-loading");
                ctx.canvas.width = this.width;
                ctx.canvas.height = this.height;
                ctx.drawImage(this, 0, 0);

                if (Gallery._Gallery.imgDOMs[p][0].getAttribute("puzzle") != "") {
                    var w = parseInt(this.width / block);
                    var h = parseInt(this.height / block);

                    var linearToXY = function(idx) {
                        var y = parseInt(idx / w);
                        var x = idx - y * w;
                        return [x, y];
                    }

                    var remapping = [];
                    for (var i = 0; i < w * h; i++) remapping[i] = i;

                    var mapping = generateList(w * h, w * h, 0xc0ffee);
                    for (var i in mapping) {
                        var tmp = remapping[i];
                        remapping[i] = remapping[mapping[i]];
                        remapping[mapping[i]] = tmp;
                    }

                    for (var i in remapping) {
                        var dxy = linearToXY(i);
                        var dx = dxy[0] * block;
                        var dy = dxy[1] * block;

                        var xy = linearToXY(remapping[i]);
                        var x = xy[0] * block;
                        var y = xy[1] * block;

                        ctx.drawImage(this, dx, dy, block, block, x, y, block, block);
                    }
                }

                var pager = etc.get(".gallery-pager");
                for (var i = 0; i < pager.length; i++) pager[i].value = p;

                document.documentElement.scrollTop = oldTop;
                isLoading = false;
            };

            img.src = Gallery._Gallery.imgList[p];

            loading.style.width = (gi.clientWidth ? gi.clientWidth : 64) + "px";
            loading.style.height = (gi.clientHeight ? gi.clientHeight : 64) + "px";

            if (img) etc.let.show("gallery-loading");
        },
    }
})();