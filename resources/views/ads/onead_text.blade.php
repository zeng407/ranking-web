@if(!is_skip_ad())
<div id="div-onead-draft"></div>
@push('scripts')
<script type="text/javascript">
  document.addEventListener('DOMContentLoaded', function() {
    var custom_call = function (params) {
        if (params.hasAd) {
          console.log('TD has AD')
        } else {
          console.log('TD AD Empty')
        }
      }
    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = "2000374";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-draft");
    ONEAD_TEXT.pub.player_mode = "text-drive";
    ONEAD_TEXT.pub.queryAdCallback = custom_call;
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);

    var script = document.createElement('script');
    script.src = 'https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js';
    document.body.appendChild(script);
  });
</script>
@endpush
@endif
