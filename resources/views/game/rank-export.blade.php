@extends('layouts.app', [
    'title' => $post->title . ' - ' . __('title.rank'),
    'ogTitle' => $post->title . ' - ' . __('title.rank'),
    'ogImage' => $ogElement?->getLowThumbUrl(),
    'ogDescription' => $post->description,
    'embed' => $embed,
    'stickyNav' => 'sticky-top-desktop',
])

@section('header')
  <style>
      .tech-button:hover {
          box-shadow: 0 0 30px rgba(0, 212, 255, 0.9), inset 0 0 20px rgba(255, 255, 255, 0.2) !important;
          transform: translateY(-2px);
      }
      .tech-button:active {
          transform: translateY(0);
      }
      [v-cloak] { display: none; }
  </style>
@endsection

@section('content')
  <rank-export inline-template
    post-serial="{{ $post->serial }}"
    :game-result="{{ $gameResult ? json_encode($gameResult) : 'null' }}"
    request-host="{{ request()->getHost() }}">

    <div class="container py-4" v-cloak>
        <div class="row justify-content-center">
            <div class="col-12 col-md-10 col-lg-8">
                <!-- Loading State -->
                <div v-if="isGeneratingImage" class="text-center py-5">
                    <div class="spinner-border text-primary" role="status" style="width: 3rem; height: 3rem;">
                        <span class="sr-only">Loading...</span>
                    </div>
                    <p class="mt-3 text-muted" style="font-size: 1.2rem;">{{ __('Generating Ranking Image...') }}</p>
                </div>

                <!-- Result Display -->
                <div v-else-if="generatedImage" class="text-center fade-in">

                    <!-- Image -->
                    <div class="mb-4 text-center">
                        <img :src="generatedImage" class="img-fluid rounded shadow-lg" alt="Ranking Result" style="max-height: 80vh; width: auto; object-fit: contain; background: #1a1a1a; display: inline-block;">
                    </div>

                    <!-- Download Button -->
                    <div class="mb-4">
                         <button @click="handleDownload" class="btn btn-lg btn-block tech-button" style="
                            background: linear-gradient(135deg, #00d4ff 0%, #00a9ff 50%, #0088ff 100%);
                            color: #fff;
                            border: 2px solid #00d4ff;
                            padding: 12px 28px;
                            font-weight: bold;
                            border-radius: 8px;
                            cursor: pointer;
                            font-size: 16px;
                            width: 100%;
                            box-shadow: 0 0 20px rgba(0, 212, 255, 0.6), inset 0 0 20px rgba(255, 255, 255, 0.1);
                            transition: all 0.3s ease;
                            letter-spacing: 1px;">
                            <i class="fa fa-download"></i>&nbsp; @{{ $t('Download Image') }}
                        </button>
                    </div>

                    <!-- Text Area -->
                    <div class="mb-3 text-left">
                        <textarea id="rankingTextarea" class="form-control" rows="8" readonly
                            style="background: #0a0e27; color: #00d4ff; border: 2px solid #00d4ff; font-family: 'Courier New', monospace; font-size: 13px; resize: none;"
                            v-model="rankingText"></textarea>
                    </div>

                    <!-- Copy Button -->
                    <div class="mb-4">
                        <button @click="copyRankingText" class="btn btn-block btn-secondary" style="
                            background: #333; color: #ddd; border: 1px solid #555; border-radius: 6px; font-size: 14px; padding: 10px;">
                            @{{ copyButtonText }}
                        </button>
                    </div>

                </div>

                <div v-else class="text-center py-5">
                     <p>Failed to generate image or no data available.</p>
                </div>

            </div>
        </div>
    </div>
  </rank-export>
@endsection


@section('footer')
  @if (!$embed)
    @if (config('services.onead.enabled') && config('services.onead.rank_page'))
      @include('ads.rank_onead_2')
    @endif
  @endif
@endsection
