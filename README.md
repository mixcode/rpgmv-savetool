
# A tool to modify rpgmaker-mv save files RPGツクールMVのセーブデータをバックアップするツール


## basic usage

* show avaliable list of save files
```
rpgmv-savetool ls
```


* copy save 1 to a backup file
```
rpgmv-savetool cp @1 save_01.rpgarch
```

* copy the backup file to @10
```
rpgmv-savetool cp save_01.rpgarch @10
```

* move save 1 to save 9
```
rpgmv-savetool mv @1 @9
```

* remove savefile @10
```
rpgmv-savetool rm @10
```

## advanced

* copy save 1, 3, 5 to a file
```
rpgmv-savetool cp @1,3,5 save_02.rpgarch
```

* copy all savefiles in backup file to save 10, 11, 12, ...
```
rpgmv-savetool cp save_02.rpgarch @10-
```

* remove all savefiles which's number is larger than 19
```
rpgmv-savetool rm @20-
```

