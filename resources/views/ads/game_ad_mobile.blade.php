<script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous">
</script>

<!-- ad_in_game -->
<ins class="adsbygoogle"
     style="display:inline-block;width:300px;height:100px;{{ app()->isProduction() ? '' : 'background-color: red;' }}"
     data-ad-client="{{ config('services.google_ad.publisher_id') }}"
     data-ad-slot="{{config('services.google_ad.game_page_ad_2_slot')}}"></ins>

@include('ads.script_load_ad')