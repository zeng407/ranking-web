
function wait(time) {
    return new Promise(resolve => {
        setTimeout(resolve, time);
    });
}

const thumbs = document.getElementsByClassName('thumb');
let src = [];
for (var i = 2; i < thumbs.length; i++) {
    console.log(i);

    //title
    let title = thumbs[i].parentElement.getElementsByTagName('td')[2].textContent;
    title = title.replace(','," ");
    //image
    if(thumbs[i] && thumbs[i].children[0] && thumbs[i].children[0].href){
        src.push(thumbs[i].children[0].href + " " + title);
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
                src.push(schema.origin + schema.pathname + " " + title);
            }
        }
        await wait(500);
        thumbs[i].click();
    }
}

src.join(',')
