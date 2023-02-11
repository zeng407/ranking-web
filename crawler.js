
function wait(time) {
    return new Promise(resolve => {
        setTimeout(resolve, time);
    });
}

const thumbs = document.getElementsByClassName('thumb');
let src = [];
for (var i = 2; i <= thumbs.length; i++) {
    console.log(i);
    //image
    if(thumbs[i] && thumbs[i].children[0] && thumbs[i].children[0].href){
        src.push(thumbs[i].children[0].href);
        await wait(200);
    }

    //video
    else if(thumbs[i]) {
        thumbs[i].click();
        await wait(1000);
        const iframes = document.getElementsByTagName('iframe');
        if(iframes[1] && iframes[1].contentDocument){
            contentDoc = iframes[1].contentDocument;
            if(contentDoc.getElementsByTagName('iframe')[0]){
                url = contentDoc.getElementsByTagName('iframe')[0].src;
                schema = new URL(url);
                src.push(schema.origin + schema.pathname);
            }
        }
        await wait(300);
        thumbs[i].click();
    }
}

src.join(',')
