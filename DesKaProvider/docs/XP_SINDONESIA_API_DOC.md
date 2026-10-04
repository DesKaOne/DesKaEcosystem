Trx# APISebelum menggunakan layanan web API pastikan anda sudah membaca seluruh Ketentuan dan Persyaratan yang ada. Simpan baik-baik dan pergunakan KEY & API sesuai kebutuhan anda. Untuk mengaktifkan atau menonaktifkan layanan ini silahkan Login dan masuk ke menu Setting. Jika anda merubah Password maka API tidak akan dapat digunakan dan anda harus menghapus API lama diganti dengan API baru.

Request API yang tersedia untuk saat ini:
1. Cek Saldo
2. Cek Harga Produk
3. List Harga Produk
4. Request Deposit
5. Cek Deposit
6. Order Produk
Semua data dikirim menggunakan metode POST dan server membalas dengan format Json


Success & Error Response:
Success:
{"success": "1", "...

Error:
{"success": "0", "error": "...

Cek Saldo (https://xp.sindonesia.net/api/saldo.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$url = "https://xp.sindonesia.net/api/saldo.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Contoh response Success:

{
  "success": "1",
  "id": "1",
  "saldo": "123"
}

Cek Harga Produk (https://xp.sindonesia.net/api/harga.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$kode="i5"; //Kode Produk
$url = "https://xp.sindonesia.net/api/harga.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api."&kode=".$kode);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Contoh response Success:

{
  "success": "1",
  "id": "1",
  "kode": "i5",
  "status": "ready",
  "harga": "6100"
}

List Harga Produk (https://xp.sindonesia.net/api/daftar_harga.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$url = "https://xp.sindonesia.net/api/daftar_harga.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Catatan: Keterangan Status 1 (adalah produk Ready) sedangkan Status 0 atau 2 (adalah produk Kosong/ Gangguan)

Request Deposit (https://xp.sindonesia.net/api/deposit.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$jumlah="1000"; //Jumlah Request Deposit (minimum 1000)
$metode="bitcoin"; //Hanya tersedia via (bitcoin, dogecoin, litecoin, bca, bni, bri, mandiri, indosat, telkomsel, xlaxiata)
$pengirim="08562622xXx"; //Nomor pengirim (wajib) hanya untuk Request Deposit via Pulsa
$url = "https://xp.sindonesia.net/api/deposit.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api."&metode=".$metode."&jumlah=".$jumlah."&pengirim=".$pengirim);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Contoh response Success Bitcoin:

{
  "success": "1",
  "trx": "1234",
  "metode": "bitcoin",
  "perkiraan_kirim": "0.0333",
  "address": "1LkD39Xa7brS6CuGmw39ffDjDR4qifp333"
}
Contoh response Success BCA:

{
  "success": "1",
  "trx": "1234",
  "metode": "bca",
  "jumlah_kirim": "12345",
  "rekening": "0130 xXx xXx",
  "atas_nama": "xXx"
}
Contoh response Success Indosat:

{
  "success": "1",
  "trx": "1234",
  "metode": "indosat",
  "perkiraan_kirim": "12345",
  "tujuan": "08512-3456-xXx",
  "pengirim": "08562622xXx"
}

Cek Deposit (https://xp.sindonesia.net/api/cek_deposit.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$trx=""; //Trx Deposit
$url = "https://xp.sindonesia.net/api/cek_deposit.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api."&trx=".$trx);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Contoh response Success:

{
  "success": "1",
  "trx": "1234",
  "status": "done",
  "jumlah": "1000"
}

Order Produk (https://xp.sindonesia.net/api/order.php)
Contoh PHP:

$id=""; //ID Member hanya angka
$key=""; //KEY 
$api=""; //API
$clb="http://domainwebsiteanda/getresponseorder.php"; //URL Callback untuk mendapatkan balasan status order
$trx="123"; //Nomor transaksi dibuat (unique)
$kod="i5"; //Kode produk
$isi="08562622xxx"; //Nomor HP/ Nomor Meter Token PLN yang diisi
$sms=""; //Diisi Nomor HP jika ingin mengirimkan SN/ Code Voucher kepada pembeli via SMS (Jenis produk voucher dan token PLN)
$url = "https://xp.sindonesia.net/api/order.php";
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $url);
curl_setopt($ch, CURLOPT_POSTFIELDS, "id=".$id."&key=".$key."&api=".$api."&url=".urlencode($clb)."&trx=".$trx."&kod=".$kod."&isi=".$isi."&sms=".$sms);
curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
$output = curl_exec($ch);
curl_close($ch);
echo $output;
Contoh response Success (diproses):

{
  "success": "1",
  "status": "proses",
  "trx": "123",
  "kode": "i5",
  "isi": "08562622xxx",
  "harga": "5700"
}
Contoh response Error Already (gagal):

Catatan: Jika Anda mengaktifkan refund otomatis di menu Alert maka response gagal tidak terbaca karena sudah terhapus otomatis
{
  "success": "0",
  "error": "already",
  "status": "gagal (batalkan manual di hisoty order)",
  "trx": "123",
  "kode": "i5",
  "isi": "08562622xxx",
  "harga": "5700",
  "time": "16 march 2015, 06:18 am"
}
Contoh response Error Already (kosong):

Catatan: Jika Anda mengaktifkan refund otomatis di menu Alert maka response kosong tidak terbaca karena sudah terhapus otomatis
{
  "success": "0",
  "error": "already",
  "status": "kosong (batalkan manual di hisoty order)",
  "trx": "123",
  "kode": "i5",
  "isi": "08562622xxx",
  "harga": "5700",
  "time": "16 march 2015, 06:18 am"
}
Contoh response Error Already (proses):

{
  "success": "0",
  "error": "already",
  "status": "proses",
  "trx": "123",
  "kode": "i5",
  "isi": "08562622xxx",
  "harga": "5700",
  "time": "16 march 2015, 06:18 am"
}
Contoh response Error Already (lambat):

{
  "success": "0",
  "error": "already",
  "status": "lambat",
  "trx": "123",
  "kode": "i5",
  "isi": "08562622xxx",
  "harga": "5700",
  "time": "16 march 2015, 06:18 am"
}
Contoh response Error Already (sukses):

{
  "success": "0",
  "error": "already",
  "status": "sukses",
  "trx": "123",
  "kode": "i5",
  "isi": "089602471xxx",
  "harga": "5700",
  "sn": "0126132123113321302",
  "time": "16 march 2015, 06:18 am"
}

Contoh Simple Setting Callback (http://domainwebsiteanda/getresponseorder.php)
Data diterima menggunakan metode GET

$id=""; //ID Member hanya angka
$key=""; //KEY 
$get_id = $_GET['id'];
$get_key = $_GET['key'];
$get_trx = $_GET['trx'];
$get_status = $_GET['status'];
$get_kod = $_GET['kod'];
$get_isi = $_GET['isi'];
$get_sn = $_GET['sn'];
if($id==$get_id && $key==$get_key){

	foreach($pdo->query("SELECT * FROM tb_tes WHERE nmr='$get_trx' AND kode='$get_kod' AND isi='$get_isi' LIMIT 1") as $a) {
	
		//Response callback yang dikirim hanya dua macam (sukses atau gagal)
		if($get_status=='sukses' && $get_trx==$a['nmr']){
		//update simpan SN
		$pdo->exec("UPDATE tb_tes SET status='SUKSES' WHERE nmr='$get_trx'");
		$pdo->exec("UPDATE tb_tes SET sn='$get_sn' WHERE nmr='$get_trx'");
		echo "sukses*ok*"; //Response kirim ke server jangan dirubah, bahwa data status (sukses) telah diterima
		}elseif($get_status=='gagal' && $get_trx==$a['nmr']){
		//trx gagal, kode salah/ nomor salah/ tidak aktif/ gangguan provider
		//saldo xp dikembalikan utuh (baca catatan di bawah)
		$pdo->exec("UPDATE tb_tes SET status='GAGAL' WHERE nmr='$get_trx'");
		echo "gagal*ok*"; //Response kirim ke server jangan dirubah, bahwa data status (gagal) telah diterima
		}
	}
}
/*
CATATAN
=======
Jika dalam 3x Callback tidak ada Response dari server Anda maka Callback tidak akan dikirim lagi
dan trx Order status Gagal dapat dibatalkan secara manual di menu History Order XPS

Nomor trx# yang ada di History Order XPS BUKANlah Trx Order API yang Anda kirim
melainkan trx# XPS, sedangkan Trx Order API yang Anda kirim dihiden (tidak ditampilkan)
*/
