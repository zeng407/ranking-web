<?php

namespace App\Console\Commands;

use App\Services\InterfaceOauthService;
use App\Services\TwitchService;
use Illuminate\Console\Command;

class RefreshAccessToken extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'refresh:token {name}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Refresh the access token for the third party service';

    protected $services = [
        'twitch' => TwitchService::class,
    ];
    /**
     * Create a new command instance.
     *
     * @return void
     */
    public function __construct()
    {
        parent::__construct();
    }

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        $name = $this->argument('name');
        if (!array_key_exists($name, $this->services)) {
            $this->error('Service not found');
            return 1;
        }
        $service = app($this->services[$name]);
        if($service instanceof InterfaceOauthService === false) {
            $this->error('Service does not implement InterfaceOauthService');
            return 1;
        }

        $service->refreshAccessToken();
        return 0;
    }
}
