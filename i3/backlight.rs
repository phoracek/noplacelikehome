use std::env;
use std::fs;

static BRIGHTNESS: &str = "/sys/class/backlight/intel_backlight/brightness";

fn main() {
    let contents = fs::read_to_string(BRIGHTNESS)
        .expect("should be able to read contents of the brightness file");

    let current_brightness: i32 = contents
        .trim()
        .parse()
        .expect("contents of the file should be an integer");

    let args: Vec<String> = env::args().collect();
    let new_brightness = match &args[1][..] {
        "up" => current_brightness + 500,
        "down" => current_brightness - 500,
        _ => current_brightness,
    };

    fs::write(BRIGHTNESS, new_brightness.to_string())
        .expect("should be able to set the new brigtness by writting to the file");
}
