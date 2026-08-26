@php
  $togawaAdUnit = config('services.google_ad.togawa_html.ad_unit');
  $togawaSlotId = 'div-gpt-ad-togawa-300x250';
@endphp

@if (config('services.google_ad.enabled') &&
        config('services.google_ad.togawa_html.enabled') &&
        $togawaAdUnit &&
        !is_skip_ad())
  <div class="gam-togawa-ad d-flex justify-content-center my-3 w-100" aria-label="Advertisement">
    <div id="{{ $togawaSlotId }}"
      style="width: 300px; min-width: 300px; min-height: 250px; display: block;">
    </div>
  </div>

  @push('scripts')
    <script data-cfasync="false">
      window.googletag = window.googletag || { cmd: [] };
      googletag.cmd.push(function() {
        var slotId = @json($togawaSlotId);
        var container = document.getElementById(slotId);

        if (!container) {
          return;
        }

        var wrapper = container.closest('.gam-togawa-ad');
        var slot = googletag.defineSlot(@json($togawaAdUnit), [300, 250], slotId);

        if (!slot) {
          return;
        }

        slot.addService(googletag.pubads());
        googletag.pubads().addEventListener('slotRenderEnded', function(event) {
          if (event.slot === slot && event.isEmpty && wrapper) {
            wrapper.style.display = 'none';
          }
        });

        if (!window.__rankingWebGptServicesEnabled) {
          googletag.enableServices();
          window.__rankingWebGptServicesEnabled = true;
        }

        googletag.display(slotId);
      });
    </script>
  @endpush
@endif
