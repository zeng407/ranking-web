<?php

namespace Tests\Browser;

use Illuminate\Foundation\Testing\DatabaseMigrations;
use Laravel\Dusk\Browser;
use Tests\DuskTestCase;
use Tests\TestHelper;

class ExampleTest extends DuskTestCase
{
    use TestHelper;

    /**
     * A basic browser test example.
     *
     * @return void
     */
    public function testHome()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 8);
        $this->browse(function (Browser $browser) use($post){;
            $browser->visit('/')
                    ->assertSee($post->title)
                    ->assertSee($post->description);
        });
    }
}
