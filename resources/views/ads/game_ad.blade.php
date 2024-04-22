<script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous">
</script>

<!-- ad_in_game -->
<ins class="adsbygoogle" 
    style="display:block;" 
    data-ad-client="{{ config('services.google_ad.publisher_id') }}"
    data-ad-slot="{{ config('services.google_ad.game_page_ad_1_slot') }}" 
    data-ad-format="auto"
    data-full-width-responsive="true"></ins>

<script>
    let retry = 10;
    let interval = setInterval(() => {
        try {
            console.log('try to push ad, try: ' + retry);
            retry--;
            (adsbygoogle = window.adsbygoogle || []).push({});
        } catch (e) {
            if (e.message.includes(
                    `All 'ins' elements in the DOM with class=adsbygoogle already have ads in them`)) {
                clearInterval(interval);
                return;
            }
        }
        if (retry <= 0) {
            clearInterval(interval);
        }
    }, 500);
</script>