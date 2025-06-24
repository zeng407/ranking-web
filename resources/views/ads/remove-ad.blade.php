@if (!is_skip_ad())
  <div id="remove-onead-ad-container" style="width: 400px">
    <div id="div-onead-draft-01"></div>
    <div id="div-onead-nd-01"></div>
  </div>

  <script type="text/javascript">
    var custom_call = function(params) {
      if (params.hasAd) {
        console.log('Ad loaded successfully');
        window._onead_ad_loaded = true;
        // 動態插入右下角按鈕
        if (!document.getElementById('remove-ad-24hr-btn-container')) {
          var btnDiv = document.createElement('div');
          btnDiv.id = 'remove-ad-24hr-btn-container';
          btnDiv.style.position = 'fixed';
          btnDiv.style.right = '20px';
          btnDiv.style.bottom = '80px';
          btnDiv.style.zIndex = '2000';
          btnDiv.innerHTML =
            '<button class="btn btn-secondary rounded-pill shadow" id="remove-ad-24hr-btn">移除廣告24hr</button>';
          document.body.appendChild(btnDiv);
          document.getElementById('remove-ad-24hr-btn').addEventListener('click', function() {
            showRemoveAdBackdrop();
          });
        }
      } else {
        console.log('No ad available at this time');
        window._onead_ad_loaded = false;
        var existBtn = document.getElementById('remove-ad-24hr-btn-container');
        if (existBtn) existBtn.remove();
      }
    }

    // 建立modal結構
    function ensureRemoveAdModal() {
      if (!document.getElementById('remove-ad-modal')) {
        var modalHtml = `
      <div class="modal fade" id="remove-ad-modal" tabindex="-1" role="dialog" aria-labelledby="removeAdModalLabel" aria-hidden="true">
        <div class="modal-dialog" role="document">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title" id="removeAdModalLabel">移除廣告 24 小時</h5>
              <button type="button" class="btn btn-outline-secondary" data-dismiss="modal" aria-label="Close">
                <span aria-hidden="true">&times;</span>放棄獎勵
              </button>
            </div>
            <div class="modal-body" id="remove-ad-modal-body">
              <div class="text-center mb-3" style="font-weight:bold;">點擊廣告獲得獎勵</div>
            </div>
          </div>
        </div>
      </div>`;
        var modalDiv = document.createElement('div');
        modalDiv.innerHTML = modalHtml;
        document.body.appendChild(modalDiv);
      }
    }

    // 顯示modal並將 remove-onead-ad-container 移入modal
    function showRemoveAdModal() {
      ensureRemoveAdModal();
      var modalBody = document.getElementById('remove-ad-modal-body');
      var adContainer = document.getElementById('remove-onead-ad-container');
      if (modalBody && adContainer) {
        modalBody.appendChild(adContainer);
      }
      var modal = document.getElementById('remove-ad-modal');
      $('#remove-ad-modal').modal('show');
      $('#remove-ad-modal').on('hidden.bs.modal', function() {
        // 1. 讓目前聚焦的元素失焦
        if (document.activeElement) document.activeElement.blur();

        // 2. 若有 aria-hidden 或 inert，移除
        var modal = document.getElementById('remove-ad-modal');
        if (modal) {
          modal.removeAttribute('aria-hidden');
          modal.removeAttribute('inert');
        }
        var existBtn = document.getElementById('remove-ad-24hr-btn-container');
        if (existBtn) existBtn.remove();
        document.body.style.overflow = 'hidden';
        console.log('Modal closed, ad container moved back to body');
      });
      $('#remove-ad-modal').on('show.bs.modal', function() {
        if (modal) modal.removeAttribute('inert');
      });
    }

    // 顯示自訂 backdrop，蓋住全畫面並顯示 remove-onead-ad-container 及提示
    function showRemoveAdBackdrop() {
      if (document.getElementById('remove-ad-backdrop')) return;
      // 建立黯淡背景
      var backdrop = document.createElement('div');
      backdrop.id = 'remove-ad-backdrop';
      backdrop.style.position = 'fixed';
      backdrop.style.top = '0';
      backdrop.style.left = '0';
      backdrop.style.width = '100vw';
      backdrop.style.height = '100vh';
      backdrop.style.background = 'rgba(0,0,0,0.5)';
      backdrop.style.zIndex = '3000';
      backdrop.style.display = 'flex';
      backdrop.style.alignItems = 'center';
      backdrop.style.justifyContent = 'center';
      // 內容區塊
      var content = document.createElement('div');
      content.style.background = '#fff';
      content.style.borderRadius = '12px';
      content.style.boxShadow = '0 2px 16px rgba(0,0,0,0.2)';
      content.style.padding = '32px 24px 24px 24px';
      content.style.width = '500px';
      content.style.height = '400px';
      content.style.maxWidth = '95vw';
      content.style.maxHeight = '90vh';
      content.style.overflow = 'auto';
      content.style.position = 'relative';
      // 關閉按鈕
      var closeBtn = document.createElement('button');
      closeBtn.innerHTML = '✕ 放棄獎勵';
      closeBtn.setAttribute('aria-label', 'Close');
      closeBtn.setAttribute('class', 'btn btn-outline-secondary d-block ml-auto mb-2');

      closeBtn.onclick = function() {
        var adContainer = document.getElementById('remove-onead-ad-container');
        if (adContainer) document.body.appendChild(adContainer);
        backdrop.remove();
        // 關閉時也移除右下角按鈕
        var btnDiv = document.getElementById('remove-ad-24hr-btn-container');
        if (btnDiv) btnDiv.remove();
        setTimeout(function() {
          var anyModalOpen = document.querySelector('.modal.show, .modal[style*="display: block"]');
          if (anyModalOpen) {
            document.body.classList.add('modal-open');
          }
        }, 50);
      };
      content.appendChild(closeBtn);
      // 提示文字
      var tip = document.createElement('div');
      tip.id = 'remove-ad-tip';
      tip.innerText = '點擊廣告完成任務';
      tip.style.textAlign = 'center';
      tip.style.fontWeight = 'bold';
      tip.style.marginBottom = '18px';
      content.appendChild(tip);
      // 將 remove-onead-ad-container 移入 backdrop 內容
      var adContainer = document.getElementById('remove-onead-ad-container');
      if (adContainer) content.appendChild(adContainer);
      backdrop.appendChild(content);
      document.body.appendChild(backdrop);
    }

    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = "2000374";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-draft-01");
    ONEAD_TEXT.pub.player_mode = "text-drive";
    ONEAD_TEXT.pub.queryAdCallback = custom_call;
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);

    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = "2000374";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-nd-01");
    ONEAD_TEXT.pub.player_mode = "native-drive";
    ONEAD_TEXT.pub.player_mode_div = "div-onead-ad";
    ONEAD_TEXT.pub.max_threads = 3;
    ONEAD_TEXT.pub.position_id = /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent) ? "5" : "0";
    ONEAD_TEXT.pub.queryAdCallback = custom_call;
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);

    // 廣告點擊監聽
    document.addEventListener('DOMContentLoaded', function() {
      var adContainer = document.getElementById('remove-onead-ad-container');

      if (adContainer) {
        var clickable = adContainer.querySelectorAll('a, div, iframe');
        clickable.forEach(function(el) {
          el.addEventListener('click', function handleAdClick(e) {
            e.preventDefault();

            fetch('/api/remove-ad-24hr', {
              method: 'POST',
              headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'X-CSRF-TOKEN': document.querySelector('meta[name="csrf-token"]').getAttribute('content')
              }
            }).then(function() {
              var tipEl = document.getElementById('remove-ad-tip');
              if (tipEl) tipEl.innerText = '感謝您的支持，重整後生效';
              var reloadBtn = document.createElement('button');
              reloadBtn.innerHTML = '<i class="fas fa-redo mr-1"></i>重新整理';
              reloadBtn.className = 'btn btn-outline-secondary btn-sm';
              reloadBtn.style.display = 'block';
              reloadBtn.style.margin = '12px auto 0 auto';
              reloadBtn.onclick = function() {
                window.location.reload();
              };
              if (tipEl && tipEl.parentNode) {
                tipEl.parentNode.insertBefore(reloadBtn, tipEl.nextSibling);
              }

              // 嘗試取得連結
              var href = el.tagName === 'A' ? el.href : (el.querySelector('a') ? el.querySelector('a')
                .href :
                null);
              if (href) {
                window.open(href, '_blank');
              }
            });
            
            clickable.forEach(function(c) {
              c.removeEventListener('click', handleAdClick);
            });
          });
        });
      }
    });
  </script>
  <script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
@endif
