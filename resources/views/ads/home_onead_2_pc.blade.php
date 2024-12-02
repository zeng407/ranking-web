<div id="div-onead-ndg"></div>
<script type="text/javascript">
  var custom_call_NDG = function (params) {
      if (params.hasAd) {
        console.log('NDG has AD')
      } else {
        console.log('NDG AD Empty');
      }
    }
  ONEAD_TEXT = {};
  ONEAD_TEXT.pub = {};
  ONEAD_TEXT.pub.uid = "2000374";
  ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-ndg");
  ONEAD_TEXT.pub.player_mode = "native-drive-group";
  ONEAD_TEXT.pub.space_id = "";
  ONEAD_TEXT.pub.position_id = "0";
  ONEAD_TEXT.pub.queryAdCallback = custom_call_NDG;
  window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
  ONEAD_text_pubs.push(ONEAD_TEXT);
</script>
<script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
