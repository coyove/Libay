curl -XPOST -d "code_url=https://www.libay.nl/assets/main.js&utf8=on" http://marijnhaverbeke.nl//uglifyjs > main.min.js
rsync -r -P --exclude codemirror ./ root@cdn.libay.nl:/var/www/libay_cdn/assets
