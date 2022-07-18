package functions;

import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;
import java.util.function.Function;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.reactive.function.client.WebClient;
import org.yaml.snakeyaml.Yaml;
import reactor.core.publisher.Mono;

@SpringBootApplication
public class CloudFunctionApplication {

  private static final Logger log = LoggerFactory.getLogger(CloudFunctionApplication.class);

  public static void main(String[] args) {
    SpringApplication.run(CloudFunctionApplication.class, args);
  }

  @Autowired
  private WebClient.Builder webClient;

  @Bean
  public Function<Map<String, Object>, Mono<Map<String, Object>>> reconcile() {
    return (resource) -> {
      Map<String, Object> parent = (Map<String, Object>) resource.get("parent");
      Map<String, Object> parentMetadata = (Map<String, Object>) parent.get("metadata");
      Map<String, Object> parentSpec = (Map<String, Object>) parent.get("spec");

      log.info("Reconciling Resource: " + parent.get("apiVersion") + "/" + parent.get("Kind") + " > " + parentMetadata.get("name"));

      boolean productionTestEnabled = (boolean) parentSpec.get("production-test-enabled");

      Map<String, Object> desiredState = new HashMap<>();

      String protocol = "https://www.";
      return Mono.zip(getServiceInfo(protocol + "google.com"),
          getServiceInfo(protocol + "amazon.com"),
          getServiceInfo(protocol + "ebay.com"))
        .map(serviceInfos -> {
          log.info("Service Infos: " + serviceInfos);
          Map<String, Object> status = new HashMap<>();
          boolean googleReady = false;
          boolean ebayReady = false;
          boolean amazonReady = false;

          if (!serviceInfos.getT1().contains("N/A") && !serviceInfos.getT1().isEmpty()) {
            googleReady = true;
          }
          if (!serviceInfos.getT2().contains("N/A") && !serviceInfos.getT2().isEmpty()) {
            amazonReady = true;
          }
          if (!serviceInfos.getT3().contains("N/A") && !serviceInfos.getT3().isEmpty()) {
            ebayReady = true;
          }

          status.put("google-ok", googleReady);
          status.put("amazon-ok", amazonReady);
          status.put("ebay-ok", ebayReady);

          status.put("prod-tests", productionTestEnabled);

          boolean internetReady = false;
          if (googleReady && amazonReady && ebayReady) {
            internetReady = true;
            if (productionTestEnabled) {
              Map<String, Object> deployment = createProductionTestDeployment();
              desiredState.put("children", Arrays.asList(deployment));
            }
          }
          status.put("ready", internetReady);

          desiredState.put("status", status);

          log.info("> Desired State: " + desiredState);
          return desiredState;
        });
    };
  }


  public Mono<String> getServiceInfo(String url) {
    return webClient.build()
      .get()
      .uri(url)
      .accept(MediaType.APPLICATION_JSON)
      .retrieve()
      .bodyToMono(String.class)
      .map(result -> "OK")
      .onErrorResume(err -> {
        log.error("Error calling page: " + url, err);
        return Mono.just("N/A");
      });

  }

  public Map<String, Object> createProductionTestDeployment() {
    Yaml yaml = new Yaml();
    String deploymentYaml = "apiVersion: apps/v1\n" +
      "kind: Deployment\n" +
      "metadata:\n" +
      "  name: internet-production-tests\n" +
      "spec:\n" +
      "  replicas: 1\n" +
      "  selector:\n" +
      "    matchLabels:\n" +
      "      app: production-tests\n" +
      "  template:\n" +
      "    metadata:\n" +
      "      labels:\n" +
      "        app: production-tests\n" +
      "    spec:\n" +
      "      containers:\n" +
      "        - name: production-tests\n" +
      "          image: salaboy/internet-production-tests:metacontroller\n" +
      "          imagePullPolicy: Always\n";
    return yaml.load(deploymentYaml);
  }
}
