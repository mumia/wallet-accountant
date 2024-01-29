function fn() {

    // Increase timeout because initial request to recently booted up container takes a while
    karate.configure('connectTimeout', 120000);
    karate.configure('readTimeout', 120000);
    karate.configure('logPrettyResponse', true);

    return {
        baseUrl: {
            api: 'http://wallet-accountant-karate:8080'
        }
    };
}
