<script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous"></script>
<!-- ad_in_rank_page -->
<ins class="adsbygoogle" 
    style="display:block;"
    data-ad-client="{{ config('services.google_ad.publisher_id') }}"
    data-ad-slot="{{ config('services.google_ad.rank_page_ad_1_slot') }}" 
    data-ad-format="auto"
    data-full-width-responsive="true">
    <p class="d-none">{{random_emoji()}}</p>
</ins>

@include('ads.script_load_ad')