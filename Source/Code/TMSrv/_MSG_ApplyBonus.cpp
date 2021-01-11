/*
*   Copyright (C) {2015}  {Victor Klafke, Charles TheHouse}
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.
*
*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see [http://www.gnu.org/licenses/].
*
*   Contact at: victor.klafke@ecomp.ufsm.br
*/
#include "ProcessClientMessage.h"

void Exec_MSG_ApplyBonus(int conn, char *pMsg)
{
	MSG_ApplyBonus *m = (MSG_ApplyBonus*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp <= 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 20);
		return;
	}

	if (m->BonusType == 0)
	{
		if (pMob[conn].MOB.ScoreBonus > 0)
		{
			if (m->Detail >= 0 && m->Detail <= 3)
			{
				int pontos = 1;

				if (pMob[conn].MOB.ScoreBonus >= 300)
					pontos = 100;

				pMob[conn].MOB.ScoreBonus -= pontos;

				if (m->Detail == 0)
					pMob[conn].MOB.BaseScore.Str += pontos;

				if (m->Detail == 1)
				{
					pMob[conn].MOB.BaseScore.Int += pontos;
					pMob[conn].MOB.BaseScore.MaxMp += (2*pontos);
					pMob[conn].MOB.CurrentScore.MaxMp += (2*pontos);
				}

				if (m->Detail == 2)
					pMob[conn].MOB.BaseScore.Dex += pontos;

				if (m->Detail == 3)
				{
					pMob[conn].MOB.BaseScore.Con += pontos;
					pMob[conn].MOB.BaseScore.MaxHp += (2*pontos);
					pMob[conn].MOB.CurrentScore.MaxHp += (2*pontos);
				}

				pMob[conn].GetCurrentScore(conn);
				SendEtc(conn);
				SendScore(conn);

				sprintf(temp, "etc,applybonus name:%s bonustype:%d detail:%d pontos:%d", pMob[conn].MOB.MobName, m->BonusType, m->Detail, pontos);
				Log(temp, pUser[conn].AccountName, 0);
				return;
			}
			else
			{
				Log("err,applybonus skill out of bound", pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
		}
		else
		{
			SendScore(conn);
			return;
		}
	}
	else if (m->BonusType == 1)
	{
		if (pMob[conn].MOB.SpecialBonus > 0)
		{
			if (m->Detail >= 0 && m->Detail <= 3)
			{
				int max_special_level = 3 * (pMob[conn].MOB.BaseScore.Level + 1);

				int max_special = 200;

				if((1 << 7) & pMob[conn].MOB.LearnedSkill && m->Detail == 1 || (1 << 15) & pMob[conn].MOB.LearnedSkill && m->Detail == 2 || (1 << 23) & pMob[conn].MOB.LearnedSkill && m->Detail == 3)
					max_special = 255;

				if (pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL)
					max_special_level += 3 * 400;

				if (pMob[conn].MOB.BaseScore.Special[m->Detail] < (max_special_level >> 1))
				{
					if (pMob[conn].MOB.BaseScore.Special[m->Detail] < max_special)
					{
						pMob[conn].MOB.SpecialBonus--;
						pMob[conn].MOB.BaseScore.Special[m->Detail]++;
						pMob[conn].GetCurrentScore(conn);

						SendScore(conn);
						SendEtc(conn);

						sprintf(temp, "etc,applybonus name:%s bonustype:%d special%d", pMob[conn].MOB.MobName, m->BonusType, m->Detail);
						Log(temp, pUser[conn].AccountName, 0);
					}
					else
						SendClientMessage(conn, g_pMessageStringTable[_NN_Maximum_Point_200_Now]);
				}
				else
					SendClientMessage(conn, g_pMessageStringTable[_NN_Maximum_Point_Now]);
										
			}
			else
				Log("err,applybonus Special, out of boundary", pUser[conn].AccountName, pUser[conn].IP);
		}
		else
			SendEtc(conn);
	}

	else if (m->BonusType == 2)
	{
		int MobClass = pMob[conn].MOB.Class;
		int skillclass = (m->Detail - 5000) / 24;
		int skillpos = (m->Detail - 5000) % 24;
		int MobTarget = m->TargetID;

		if (m->Detail >= 5000 && m->Detail <= 5095)
		{
			if (MobTarget >= MAX_USER && MobTarget < MAX_MOB)
			{
				if (MobClass == skillclass)
				{
					int Skillbonus = g_pSpell[skillpos + 24 * skillclass].SkillPoint;

					int myskillbonus = pMob[conn].MOB.SkillBonus;

					if (pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL)
						myskillbonus = 1500;

					if (Skillbonus <= myskillbonus)
					{
						int learn = 1 << skillpos;

						if(skillpos == 7 || skillpos == 15 || skillpos == 23)
						{
							if ((1 << 7) & pMob[conn].MOB.LearnedSkill || (1 << 15) & pMob[conn].MOB.LearnedSkill || (1 << 23) & pMob[conn].MOB.LearnedSkill)
							{
								SendClientMessage(conn, g_pMessageStringTable[_NN_Only_OneSkillLearn]);
								return;
							}

							int skillin = 0;
													
							for(int i = 0; i < 8; i++)
							{
								if ((1 << (skillpos - i)) & pMob[conn].MOB.LearnedSkill)
									skillin++;
							}

							if(skillin != 7)
							{
								SendClientMessage(conn, g_pMessageStringTable[_NN_Befor_LearnSkill]);
								return;
							}

							int coin = pMob[conn].extra.ClassMaster != MORTAL && pMob[conn].extra.ClassMaster != ARCH ? 50000000 : 50000000;

							if(skillin == 7 && pMob[conn].MOB.Coin < coin)
							{
								sprintf(temp, g_pMessageStringTable[_DN_D_Cost], coin);
								SendClientMessage(conn, temp);
								return;
							}
						}

						if ((1 << skillpos) & pMob[conn].MOB.LearnedSkill)
						{
							SendClientMessage(conn, g_pMessageStringTable[_NN_Already_Learned_It]);
							return;
						}
						else
						{
							int reqlv = pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH ? g_pItemList[m->Detail].ReqLvl : 0;
							if (pMob[conn].MOB.CurrentScore.Level >= reqlv)
							{
								if (pMob[conn].MOB.CurrentScore.Special[1] >= g_pItemList[m->Detail].ReqInt
									&& pMob[conn].MOB.CurrentScore.Special[2] >= g_pItemList[m->Detail].ReqDex
									&& pMob[conn].MOB.CurrentScore.Special[3] >= g_pItemList[m->Detail].ReqCon)
								{
									int coin = pMob[conn].extra.ClassMaster != MORTAL && pMob[conn].extra.ClassMaster != ARCH ? 50000000 : 50000000;

									if(skillpos == 7 || skillpos == 15 || skillpos == 23)
										pMob[conn].MOB.Coin -= coin;

									pMob[conn].MOB.LearnedSkill |= learn;
									pMob[conn].MOB.SkillBonus -= Skillbonus;
									pMob[conn].GetCurrentScore(conn);
									SendScore(conn);
									SendEtc(conn);

									sprintf(temp, "etc,applybonus name:%s bonustype:%d skillpos:%d", pMob[conn].MOB.MobName, m->BonusType, skillpos);
									Log(temp, pUser[conn].AccountName, 0);
								}
								else
								{
									SendClientMessage(conn, g_pMessageStringTable[_NN_Need_More_Mastery_To_Learn]);
									return;
								}
							}
							else
							{
								SendClientMessage(conn, g_pMessageStringTable[_NN_Need_More_Level_To_Learn]);
								return;
							}
						}
					}
					else
					{
						SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Enough_Skill_Point]);
						return;
					}
				}
				else
				{
					SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Learn_Other_Class_Skill]);
					return;
				}
			}
			else
			{
				Log("err,MSG_ApplyBonus, request target id is  out of bound", pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
		}
		else
		{
			Log("err,MSG_ApplyBonus, request skill out of bound", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
	}
}