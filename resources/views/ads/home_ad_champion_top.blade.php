<script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous"></script>
<!-- ad_in_home@responsive -->
<ins class="adsbygoogle" 
    style="display:inline-block;width:200px;height:200px"
    data-ad-client="{{ config('services.google_ad.publisher_id') }}"
    data-ad-slot="{{ config('services.google_ad.home_page_champion_top_slot') }}">
</ins>

@include('ads.script_load_ad')