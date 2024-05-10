<script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-3442386930660042"
     crossorigin="anonymous"></script>
<!-- ad_in_game_mobile@responsive -->
<ins class="adsbygoogle"
     style="display:block;" 
     data-ad-client="{{ config('services.google_ad.publisher_id') }}"
     data-ad-slot="{{config('services.google_ad.game_page_ad_3_slot')}}"
     data-ad-format="auto"
     data-full-width-responsive="true">
     @include('ads.random_emjoi')
</ins>
@include('ads.script_load_ad')
