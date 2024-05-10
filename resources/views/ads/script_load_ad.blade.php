<script>
    function loadAd(retryCount, intervalDelay) {
        let retry = retryCount;
        let interval = setInterval(() => {
            try {
                // console.log('try to push ad, try: ' + retry);
                retry--;
                (adsbygoogle = window.adsbygoogle || []).push({});

                //if ad is loaded, then it will throw an error
                (adsbygoogle = window.adsbygoogle || []).push({});
            } catch (e) {
                if (e.message.includes(`All 'ins' elements in the DOM with class=adsbygoogle already have ads in them`)) {
                    // after loaded ad, send a event to Home.vue
                    window.dispatchEvent(new Event('ad-loaded'));
                    clearInterval(interval);
                    return;
                }
            }
            if (retry <= 0) {
                clearInterval(interval);
            }
        }, intervalDelay);
    }

    loadAd(10, 500);
</script>
