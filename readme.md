# Note on Libay

Example config is shown here:

```json
{
    "Connect": "user=... port=... password=... dbname=...",
    "Salt": "...",
    "Listen": "...",
    "CDNPrefix": "https://cdn.libay.nl",
    "Host": "https://www.libay.nl",
    "DebugHost": "",
    "Referer": "(?i)^https?:\\/\\/(www.)?libay\\.nl",
    "Description": "Libay Blog",
    "Title": "Libay",
    "Author": "libay",
    "Email": "hi@libay.nl",
    "AnonymousArea": 65535,
    "ReplyArea": 65536,
    "MessageArea": 65534,
    "AllowRegistration": true,
    "ImagesAllowed": [
        "admin",
        "user",
        "superuser"
    ],
    "PostsAllowed": [
        "",
        "admin",
        "user",
        "superuser"
    ],
    "ArticlesPerPage": 15,
    "MaxRevision": 10,
    "CacheLifetime": -1,
    "CacheEntities": 10,
    "Tags": null,
    "AdminPassword": "...",
    "MaxImageSize": 10,
    "MaxImageSizeGuest": 4,
    "MaxArticleContentLength": 256,
    "PlaygroundMaxImages": 50,
    "AllowAnonymousUpload": false,
    "HTMLTags": {
        "strike": true, "img": true, "p": true, "ol": true, "ul": true, "li": true,
        "b": true, "del": true, "strong": true, "em": true, "i": true, "u": true,
        "sub": true, "sup": true, "div": true, "br": true, "hr": true, "span": true,
        "font": true, "a": true, "table": true, "tr": true, "td": true, "th": true,
        "thead": true, "tbody": true, "pre": true, "h1": true, "h2": true, "h3": true,
        "h4": true, "h5": true, "script": true
    },
    "HTMLAttrs": {
        "href": true, "target": true, "src": true, "alt": true, "title": true,
        "id": true, "class": true, "height": true, "width": true,
        "cellpadding": true, "cellspacing": true, "border": true, 
        "align": true, "valign": true, "halign": true
    },
    "Privilege": {
        "superuser": {
            "AnnounceArticle": true,
            "Cooldown": 10,
            "DeleteOthers": true,
            "EditOthers": false,
            "MakeLocked": false,
            "ViewOtherTrash": true
        }
    },
    "MaxIdleConns": 5,
    "MaxOpenConns": 20,
    "ConfigPath": "./config.json"
}
```