<script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous"></script>

<!-- home_ad_bottom -->
<ins class="adsbygoogle" 
    style="display:block;"
    data-ad-client="{{ config('services.google_ad.publisher_id') }}"
    data-ad-slot="{{ config('services.google_ad.home_page_ad_3_slot') }}" 
    data-ad-format="auto"
    data-full-width-responsive="true"></ins>
    
@include('ads.script_load_ad')
