curl -X POST -d "code_url=https://2ax.org/assets/main.js&utf8=on" http://marijnhaverbeke.nl//uglifyjs > main.min.js
curl -X POST -s --data-urlencode 'input@main.css' https://cssminifier.com/raw > main.min.css

# rsync -r -P --exclude codemirror ./ root@cdn.libay.nl:/var/www/libay_cdn/assets
